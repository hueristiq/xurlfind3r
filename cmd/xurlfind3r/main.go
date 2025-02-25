package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/internal/input"
	"github.com/hueristiq/xurlfind3r/internal/output"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v4"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.source.hueristiq.com/logger"
	"go.source.hueristiq.com/logger/formatter"
	"go.source.hueristiq.com/logger/levels"
)

var (
	au = aurora.New(aurora.WithColors(true))

	configurationFilePath string

	inputDomains             []string
	inputDomainsListFilePath string

	includeSubdomains bool

	listSources      bool
	sourcesToExclude []string
	sourcesToUse     []string

	filterPattern string
	matchPattern  string

	outputInJSONL       bool
	monochrome          bool
	outputFilePath      string
	outputDirectoryPath string
	silent              bool
	verbose             bool
)

func init() {
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.DefaultConfigurationFilePath, "")

	pflag.StringSliceVarP(&inputDomains, "domain", "d", []string{}, "")
	pflag.StringVarP(&inputDomainsListFilePath, "list", "l", "", "")

	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")

	pflag.BoolVar(&listSources, "sources", false, "")
	pflag.StringSliceVarP(&sourcesToExclude, "exclude-sources", "e", []string{}, "")
	pflag.StringSliceVarP(&sourcesToUse, "use-sources", "u", []string{}, "")

	pflag.StringVarP(&filterPattern, "filter", "f", "", "")
	pflag.StringVarP(&matchPattern, "match", "m", "", "")

	pflag.BoolVar(&outputInJSONL, "json", false, "")
	pflag.BoolVar(&monochrome, "monochrome", false, "")
	pflag.StringVarP(&outputFilePath, "output", "o", "", "")
	pflag.StringVarP(&outputDirectoryPath, "output-directory", "O", "", "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		logger.Info().Label("").Msg(configuration.BANNER(au))

		h := "USAGE:\n"
		h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.NAME)

		h += "\nCONFIGURATION:\n"

		defaultConfigurationFilePath := strings.ReplaceAll(configuration.DefaultConfigurationFilePath, configuration.UserDotConfigDirectoryPath, "$HOME/.config")

		h += fmt.Sprintf(" -c, --configuration string          configuration file path (default: %v)\n", au.Underline(defaultConfigurationFilePath).Bold())

		h += "\nINPUT:\n"
		h += " -d, --domain string[]               target domain\n"
		h += " -l, --list string                   target domains list file path\n"

		h += "\nTIP: For multiple input domains use comma(,) separated value with `-d`,\n"
		h += "     specify multiple `-d`, load from file with `-l` or load from stdin.\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool       match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "     --sources bool                  list available sources\n"
		h += " -e, --exclude-sources string[]      comma(,) separated sources to exclude\n"
		h += " -u, --use-sources string[]          comma(,) separated sources to use\n"

		h += "\nFILTER & MATCH:\n"
		h += " -f, --filter string                 regex to filter URLs\n"
		h += " -m, --match string                  regex to match URLs\n"

		h += "\nOUTPUT:\n"
		h += "     --jsonl bool                    output URLs in JSONL format\n"
		h += "     --monochrome bool               stdout monochrome output\n"
		h += " -o, --output string                 output URLs file path\n"
		h += " -O, --output-directory string       output URLs directory path\n"
		h += " -s, --silent bool                   stdout URLs only output\n"
		h += " -v, --verbose bool                  stdout verbose output\n"

		logger.Info().Label("").Msg(h)
		logger.Print().Msg("")
	}

	pflag.Parse()

	if err := configuration.CreateUpdate(configurationFilePath); err != nil {
		logger.Fatal().Msg(err.Error())
	}

	viper.SetConfigFile(configurationFilePath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix(strings.ToUpper(configuration.NAME))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		logger.Fatal().Msg(err.Error())
	}

	logger.DefaultLogger.SetFormatter(formatter.NewConsoleFormatter(&formatter.ConsoleFormatterConfiguration{
		Colorize: !monochrome,
	}))

	if verbose {
		logger.DefaultLogger.SetMaxLogLevel(levels.LevelDebug)
	}

	if silent {
		logger.DefaultLogger.SetMaxLogLevel(levels.LevelSilent)
	}

	au = aurora.New(aurora.WithColors(!monochrome))
}

func main() {
	logger.Info().Label("").Msg(configuration.BANNER(au))

	var err error

	var cfg *configuration.Configuration

	if err = viper.Unmarshal(&cfg); err != nil {
		logger.Fatal().Msg(err.Error())
	}

	if listSources {
		logger.Info().Msgf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(cfg.Sources))).Bold())
		logger.Info().Msgf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold())
		logger.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&cfg.Keys).Elem()

		for i := range keysElem.NumField() {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range cfg.Sources {
			if _, ok := needsKey[source]; ok {
				logger.Print().Msgf("> %s *", source)
			} else {
				logger.Print().Msgf("> %s", source)
			}
		}

		logger.Print().Msg("")

		os.Exit(0)
	}

	if inputDomainsListFilePath != "" {
		var file *os.File

		file, err = os.Open(inputDomainsListFilePath)
		if err != nil {
			logger.Fatal().Msg(err.Error())
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				inputDomains = append(inputDomains, domain)
			}
		}

		if err = scanner.Err(); err != nil {
			logger.Fatal().Msg(err.Error())
		}

		file.Close()
	}

	if input.HasStdin() {
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				inputDomains = append(inputDomains, domain)
			}
		}

		if err = scanner.Err(); err != nil {
			logger.Fatal().Msg(err.Error())
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
		logger.Fatal().Msg(err.Error())
	}

	outputWritter := output.NewWritter()

	if outputInJSONL {
		outputWritter.SetFormatToJSONL()
	}

	for index := range inputDomains {
		domain := inputDomains[index]

		logger.Info().Msgf("Finding URLs for %s...", au.Underline(domain).Bold())
		logger.Print().Msg("")

		writers := []io.Writer{
			os.Stdout,
		}

		var file *os.File

		switch {
		case outputFilePath != "":
			file, err = outputWritter.CreateFile(outputFilePath)
			if err != nil {
				logger.Error().Msg(err.Error())
			}

			writers = append(writers, file)
		case outputDirectoryPath != "":
			file, err = outputWritter.CreateFile(filepath.Join(outputDirectoryPath, domain))
			if err != nil {
				logger.Error().Msg(err.Error())
			}

			writers = append(writers, file)
		}

		results := finder.Find(domain)

		for result := range results {
			for index := range writers {
				writer := writers[index]

				switch result.Type {
				case sources.ResultError:
					logger.Error().Msgf("%s: %s", result.Source, result.Error)
				case sources.ResultURL:
					if err := outputWritter.Write(writer, domain, result); err != nil {
						logger.Error().Msg(err.Error())
					}
				}
			}
		}

		file.Close()

		logger.Print().Msg("")
	}
}
