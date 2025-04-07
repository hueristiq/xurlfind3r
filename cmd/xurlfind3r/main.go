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

	hqgologger "github.com/hueristiq/hq-go-logger"
	"github.com/hueristiq/hq-go-logger/formatter"
	"github.com/hueristiq/hq-go-logger/levels"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/internal/input"
	"github.com/hueristiq/xurlfind3r/internal/output"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v4"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	configurationFilePath    string
	inputDomains             []string
	inputDomainsListFilePath string
	includeSubdomains        bool
	listSources              bool
	sourcesToExclude         []string
	sourcesToUse             []string
	outputInJSONL            bool
	outputFilePath           string
	outputDirectoryPath      string
	monochrome               bool
	silent                   bool
	verbose                  bool

	au = aurora.New(aurora.WithColors(true))
)

func init() {
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.DefaultConfigurationFilePath, "")
	pflag.StringSliceVarP(&inputDomains, "domain", "d", []string{}, "")
	pflag.StringVarP(&inputDomainsListFilePath, "list", "l", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSources, "sources", false, "")
	pflag.StringSliceVarP(&sourcesToExclude, "sources-to-exclude", "e", []string{}, "")
	pflag.StringSliceVarP(&sourcesToUse, "sources-to-use", "u", []string{}, "")
	pflag.BoolVar(&outputInJSONL, "jsonl", false, "")
	pflag.StringVarP(&outputFilePath, "output", "o", "", "")
	pflag.StringVarP(&outputDirectoryPath, "output-directory", "O", "", "")
	pflag.BoolVar(&monochrome, "monochrome", false, "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.Usage = func() {
		hqgologger.Info().Label("").Msg(configuration.BANNER(au))

		h := "USAGE:\n"
		h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.NAME)

		h += "\nCONFIGURATION:\n"

		defaultConfigurationFilePath := strings.ReplaceAll(configuration.DefaultConfigurationFilePath, configuration.UserDotConfigDirectoryPath, "$HOME/.config")

		h += fmt.Sprintf(" -c, --configuration string            configuration file path (default: %v)\n", au.Underline(defaultConfigurationFilePath).Bold())

		h += "\nINPUT:\n"
		h += " -d, --domain string[]                 target domain\n"
		h += " -l, --list string                     target domains list file path\n"

		h += "\nTIP: For multiple input domains use comma(,) separated value with `-d`,\n"
		h += "     specify multiple `-d`, load from file with `-l` or load from stdin.\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool         match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "     --sources bool                    list available sources\n"
		h += " -e, --sources-to-exclude string[]     comma(,) separated sources to exclude\n"
		h += " -u, --sources-to-use string[]         comma(,) separated sources to use\n"

		h += "\nOUTPUT:\n"
		h += "     --jsonl bool                      output URLs in JSONL\n"
		h += " -o, --output string                   output URLs file path\n"
		h += " -O, --output-directory string         output URLs directory path\n"
		h += "     --monochrome bool                 stdout monochrome output\n"
		h += " -s, --silent bool                     stdout URLs only output\n"
		h += " -v, --verbose bool                    stdout verbose output\n"

		hqgologger.Info().Label("").Msg(h)
		hqgologger.Print().Msg("")
	}

	pflag.Parse()

	if err := configuration.CreateUpdate(configurationFilePath); err != nil {
		hqgologger.Fatal().Msg(err.Error())
	}

	viper.SetConfigFile(configurationFilePath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix(strings.ToUpper(configuration.NAME))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		hqgologger.Fatal().Msg(err.Error())
	}

	hqgologger.DefaultLogger.SetFormatter(formatter.NewConsoleFormatter(&formatter.ConsoleFormatterConfiguration{
		Colorize: !monochrome,
	}))

	if silent {
		hqgologger.DefaultLogger.SetMaxLogLevel(levels.LevelSilent)
	}

	if verbose {
		hqgologger.DefaultLogger.SetMaxLogLevel(levels.LevelDebug)
	}

	au = aurora.New(aurora.WithColors(!monochrome))
}

func main() {
	hqgologger.Info().Label("").Msg(configuration.BANNER(au))

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		hqgologger.Fatal().Msg(err.Error())
	}

	if listSources {
		hqgologger.Info().Msgf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(cfg.Sources))).Bold())
		hqgologger.Info().Msgf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold())
		hqgologger.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&cfg.Keys).Elem()

		for i := range keysElem.NumField() {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range cfg.Sources {
			if _, ok := needsKey[source]; ok {
				hqgologger.Print().Msgf("> %s *", source)
			} else {
				hqgologger.Print().Msgf("> %s", source)
			}
		}

		hqgologger.Print().Msg("")

		os.Exit(0)
	}

	if inputDomainsListFilePath != "" {
		file, err := os.Open(inputDomainsListFilePath)
		if err != nil {
			hqgologger.Fatal().Msg(err.Error())
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				inputDomains = append(inputDomains, domain)
			}
		}

		if err := scanner.Err(); err != nil {
			hqgologger.Fatal().Msg(err.Error())
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

		if err := scanner.Err(); err != nil {
			hqgologger.Fatal().Msg(err.Error())
		}
	}

	writer := output.NewWriter()

	if outputInJSONL {
		writer.SetFormatToJSONL()
	}

	finder, err := xurlfind3r.New(&xurlfind3r.Configuration{
		IncludeSubdomains: includeSubdomains,
		SourcesToUse:      sourcesToUse,
		SourcesToExclude:  sourcesToExclude,
		Keys:              cfg.Keys,
	})
	if err != nil {
		hqgologger.Fatal().Msg(err.Error())
	}

	for index := range inputDomains {
		domain := inputDomains[index]

		hqgologger.Info().Msgf("Finding URLs for %s...", au.Underline(domain).Bold())
		hqgologger.Print().Msg("")

		outputs := []io.Writer{
			os.Stdout,
		}

		var file *os.File

		switch {
		case outputFilePath != "":
			file, err = writer.CreateFile(outputFilePath)
			if err != nil {
				hqgologger.Error().Msg(err.Error())
			}

			outputs = append(outputs, file)
		case outputDirectoryPath != "":
			file, err = writer.CreateFile(filepath.Join(outputDirectoryPath, domain))
			if err != nil {
				hqgologger.Error().Msg(err.Error())
			}

			outputs = append(outputs, file)
		}

		results := finder.Find(domain)

		for result := range results {
			for _, output := range outputs {
				switch result.Type {
				case sources.ResultError:
					if verbose {
						hqgologger.Error().Msgf("%s: %s", result.Source, result.Error)
					}
				case sources.ResultURL:
					if err := writer.Write(output, domain, result); err != nil {
						hqgologger.Error().Msg(err.Error())
					}
				}
			}
		}

		file.Close()

		hqgologger.Print().Msg("")
	}
}
