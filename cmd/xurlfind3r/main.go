package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/hqgolog"
	"github.com/hueristiq/hqgolog/formatter"
	"github.com/hueristiq/hqgolog/levels"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/pkg/scraper"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
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
	parseWaybackRobots    bool
	parseWaybackSource    bool
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
	pflag.BoolVar(&parseWaybackRobots, "parse-wayback-robots", false, "")
	pflag.BoolVar(&parseWaybackSource, "parse-wayback-source", false, "")
	pflag.StringVarP(&filterPattern, "filter", "f", "", "")
	pflag.StringVarP(&matchPattern, "match", "m", "", "")
	pflag.BoolVar(&monochrome, "no-color", false, "")
	pflag.StringVarP(&output, "output", "o", "", "")
	pflag.StringVarP(&outputDirectory, "outputDirectory", "O", "", "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "\nUSAGE:\n"
		h += fmt.Sprintf("  %s [OPTIONS]\n", configuration.NAME)

		h += "\nCONFIGURATION:\n"
		defaultConfigurationFilePath := strings.ReplaceAll(configuration.ConfigurationFilePath, configuration.UserDotConfigDirectoryPath, "$HOME/.config")
		h += fmt.Sprintf(" -c, --configuration string          configuration file path (default: %s)\n", defaultConfigurationFilePath)

		h += "\nINPUT:\n"
		h += " -d, --domain string[]               target domain\n"
		h += " -l, --list string                   target domains' list file path\n"

		h += "\n   TIP: For multiple input domains use comma(,) separated value with `-d`,\n"
		h += "        specify multiple `-d`, load from file with `-l` or load from stdin.\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool       match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "     --sources bool                  list supported sources\n"
		h += " -u, --use-sources string[]          comma(,) separated sources to use\n"
		h += " -e, --exclude-sources string[]      comma(,) separated sources to exclude\n"
		h += "     --parse-wayback-robots bool     with wayback, parse robots.txt snapshots\n"
		h += "     --parse-wayback-source bool     with wayback, parse source code snapshots\n"

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

	// Initialize logger (hqgolog)
	hqgolog.DefaultLogger.SetMaxLevel(levels.LevelInfo)

	if verbose {
		hqgolog.DefaultLogger.SetMaxLevel(levels.LevelDebug)
	}

	hqgolog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	// Create or Update configuration
	if err := configuration.CreateUpdate(configurationFilePath); err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	au = aurora.NewAurora(!monochrome)
}

func main() {
	// print Banner.
	if !silent {
		fmt.Fprintln(os.Stderr, configuration.BANNER)
	}

	var err error

	var config configuration.Configuration

	// read in configuration.
	config, err = configuration.Read(configurationFilePath)
	if err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	// if --sources: List suported sources & exit.
	if listSources {
		hqgolog.Print().Msg("")
		hqgolog.Info().Msgf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(config.Sources))).Bold())
		hqgolog.Info().Msgf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold())
		hqgolog.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&config.Keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for i, source := range config.Sources {
			if _, ok := needsKey[source]; ok {
				hqgolog.Print().Msgf("%d. %s *", i+1, source)
			} else {
				hqgolog.Print().Msgf("%d. %s", i+1, source)
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
			hqgolog.Error().Msg(err.Error())

			return
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				domains = append(domains, domain)
			}
		}

		if err = scanner.Err(); err != nil {
			hqgolog.Error().Msg(err.Error())

			return
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
			hqgolog.Error().Msg(err.Error())

			return
		}
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

	options := &scraper.Options{
		IncludeSubdomains:  includeSubdomains,
		SourcesToUSe:       sourcesToUse,
		SourcesToExclude:   sourcesToExclude,
		Keys:               config.Keys,
		ParseWaybackRobots: parseWaybackRobots,
		ParseWaybackSource: parseWaybackSource,
		FilterPattern:      filterPattern,
		Matchattern:        matchPattern,
	}

	var spr *scraper.Finder

	spr, err = scraper.New(options)
	if err != nil {
		hqgolog.Error().Msg(err.Error())

		return
	}

	for index := range domains {
		domain := domains[index]

		if !silent {
			hqgolog.Print().Msg("")
			hqgolog.Info().Msgf("Finding URLs for %v...", au.Underline(domain).Bold())
			hqgolog.Print().Msg("")
		}

		URLs := spr.Scrape(domain)

		switch {
		case output != "":
			outputURLs(consolidatedWriter, URLs)
		case outputDirectory != "":
			var domainFile *os.File

			domainFile, err = os.OpenFile(filepath.Join(outputDirectory, domain+".txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				hqgolog.Error().Msg(err.Error())

				return
			}

			domainWriter := bufio.NewWriter(domainFile)

			outputURLs(domainWriter, URLs)
		default:
			outputURLs(nil, URLs)
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
		case sources.Error:
			if verbose {
				hqgolog.Error().Msgf("%s: %s\n", URL.Source, URL.Error)
			}
		case sources.URL:
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
