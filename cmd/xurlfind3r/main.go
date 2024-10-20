package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/hqgolog"
	"github.com/hueristiq/hqgolog/formatter"
	"github.com/hueristiq/hqgolog/levels"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	au aurora.Aurora

	configurationFilePath string
	domains               []string
	domainsListFilePath   string
	includeSubdomains     bool
	listSources           bool
	sourcesToUse          []string
	sourcesToExclude      []string
	filterPattern         string
	matchPattern          string
	monochrome            bool
	output                string
	outputDirectory       string
	silent                bool
	verbose               bool
)

func init() {
	// Handle CLI arguments, flags & help message (pflag)
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.ConfigurationFilePath, "")
	pflag.StringSliceVarP(&domains, "domain", "d", []string{}, "")
	pflag.StringVarP(&domainsListFilePath, "list", "l", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSources, "sources", false, "")
	pflag.StringSliceVarP(&sourcesToUse, "use-sources", "u", []string{}, "")
	pflag.StringSliceVarP(&sourcesToExclude, "exclude-sources", "e", []string{}, "")
	pflag.StringVarP(&filterPattern, "filter", "f", "", "")
	pflag.StringVarP(&matchPattern, "match", "m", "", "")
	pflag.BoolVar(&monochrome, "no-color", false, "")
	pflag.StringVarP(&output, "output", "o", "", "")
	pflag.StringVarP(&outputDirectory, "output-directory", "O", "", "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "\nUSAGE:\n"
		h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.NAME)

		h += "\nCONFIGURATION:\n"
		defaultConfigurationFilePath := strings.ReplaceAll(configuration.ConfigurationFilePath, configuration.UserDotConfigDirectoryPath, "$HOME/.config")
		h += fmt.Sprintf(" -c, --configuration string          configuration file (default: %s)\n", defaultConfigurationFilePath)

		h += "\nINPUT:\n"
		h += " -d, --domain string[]               target domain\n"
		h += " -l, --list string                   target domains' list file path\n"

		h += "\nTIP: For multiple input domains use comma(,) separated value with `-d`,\n"
		h += "     specify multiple `-d`, load from file with `-l` or load from stdin.\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool       match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "     --sources bool                  list supported sources\n"
		h += " -u, --use-sources string[]          comma(,) separated sources to use\n"
		h += " -e, --exclude-sources string[]      comma(,) separated sources to exclude\n"

		h += "\nFILTER & MATCH:\n"
		h += " -f, --filter string                 regex to filter URLs\n"
		h += " -m, --match string                  regex to match URLs\n"

		h += "\nOUTPUT:\n"
		h += "     --no-color bool                 disable colored output\n"
		h += " -o, --output string                 output URLs file path\n"
		h += " -O, --output-directory string       output URLs directory path\n"
		h += " -s, --silent bool                   display output subdomains only\n"
		h += " -v, --verbose bool                  display verbose output\n"

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	// Initialize configuration management (...with viper)
	if err := configuration.CreateUpdate(configurationFilePath); err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	viper.SetConfigFile(configurationFilePath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix("XURLFIND3R")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln(err)
	}

	// Initialize logger (hqgolog)
	hqgolog.DefaultLogger.SetMaxLevel(levels.LevelInfo)

	if verbose {
		hqgolog.DefaultLogger.SetMaxLevel(levels.LevelDebug)
	}

	hqgolog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	au = aurora.NewAurora(!monochrome)
}

func main() {
	// print Banner.
	if !silent {
		fmt.Fprintln(os.Stderr, configuration.BANNER)
	}

	var err error

	var cfg *configuration.Configuration

	if err = viper.Unmarshal(&cfg); err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	// if --sources: List suported sources & exit.
	if listSources {
		hqgolog.Print().Msg("")
		hqgolog.Info().Msgf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(cfg.Sources))).Bold())
		hqgolog.Info().Msgf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold())
		hqgolog.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&cfg.Keys).Elem()

		for i := range keysElem.NumField() {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range cfg.Sources {
			if _, ok := needsKey[source]; ok {
				hqgolog.Print().Msgf("> %s *", source)
			} else {
				hqgolog.Print().Msgf("> %s", source)
			}
		}

		hqgolog.Print().Msg("")

		os.Exit(0)
	}

	// load input domains from file
	if domainsListFilePath != "" {
		var file *os.File

		file, err = os.Open(domainsListFilePath)
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				domains = append(domains, domain)
			}
		}

		if err = scanner.Err(); err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}
	}

	// load input domains from stdin
	if hasStdin() {
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				domains = append(domains, domain)
			}
		}

		if err = scanner.Err(); err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}
	}

	var finder *xurlfind3r.Finder

	finder, err = xurlfind3r.New(&xurlfind3r.Configuration{
		IncludeSubdomains: includeSubdomains,
		SourcesToUse:      sourcesToUse,
		SourcesToExclude:  sourcesToExclude,
		Keys:              cfg.Keys,
		FilterPattern:     filterPattern,
		MatchPattern:      matchPattern,
	})
	if err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	// scrape and output URLs.
	var consolidatedWriter *bufio.Writer

	if output != "" {
		directory := filepath.Dir(output)

		mkdir(directory)

		var consolidatedFile *os.File

		consolidatedFile, err = os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		defer consolidatedFile.Close()

		consolidatedWriter = bufio.NewWriter(consolidatedFile)
	}

	if outputDirectory != "" {
		mkdir(outputDirectory)
	}

	for _, domain := range domains {
		if !silent {
			hqgolog.Print().Msg("")
			hqgolog.Info().Msgf("Finding URLs for %v...", au.Underline(domain).Bold())
			hqgolog.Print().Msg("")
		}

		results := finder.Find(domain)

		switch {
		case output != "":
			outputURLs(consolidatedWriter, results)
		case outputDirectory != "":
			var domainFile *os.File

			domainFile, err = os.OpenFile(filepath.Join(outputDirectory, domain+".txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}

			domainWriter := bufio.NewWriter(domainFile)

			outputURLs(domainWriter, results)
		default:
			outputURLs(nil, results)
		}
	}
}

func hasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	isPipedFromChrDev := (stat.Mode() & os.ModeCharDevice) == 0
	isPipedFromFIFO := (stat.Mode() & os.ModeNamedPipe) != 0

	return isPipedFromChrDev || isPipedFromFIFO
}

func mkdir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}
	}
}

func outputURLs(writer *bufio.Writer, URLs chan sources.Result) {
	for URL := range URLs {
		switch URL.Type {
		case sources.ResultError:
			if verbose {
				hqgolog.Error().Msgf("%s: %s\n", URL.Source, URL.Error)
			}
		case sources.ResultURL:
			if verbose {
				hqgolog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			} else {
				hqgolog.Print().Msg(URL.Value)
			}

			if writer != nil {
				fmt.Fprintln(writer, URL.Value)

				if err := writer.Flush(); err != nil {
					hqgolog.Fatal().Msg(err.Error())
				}
			}
		}
	}
}
