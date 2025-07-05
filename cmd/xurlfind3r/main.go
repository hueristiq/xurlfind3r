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
	hqgologgerformatter "github.com/hueristiq/hq-go-logger/formatter"
	hqgologgerlevels "github.com/hueristiq/hq-go-logger/levels"
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
	configurationFilePath string
	domains               []string
	domainsFilePath       string
	includeSubdomains     bool
	listSupportedSources  bool
	sourcesToUse          []string
	sourcesToExclude      []string
	outputInJSONL         bool
	outputFilePath        string
	outputDirectoryPath   string
	monochrome            bool
	silent                bool
	verbose               bool

	au = aurora.New(aurora.WithColors(true))
)

func init() {
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.DefaultConfigurationFilePath, "")
	pflag.StringSliceVarP(&domains, "domain", "d", []string{}, "")
	pflag.StringVarP(&domainsFilePath, "list", "l", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSupportedSources, "sources", false, "")
	pflag.StringSliceVarP(&sourcesToUse, "sources-to-use", "u", []string{}, "")
	pflag.StringSliceVarP(&sourcesToExclude, "sources-to-exclude", "e", []string{}, "")
	pflag.BoolVar(&outputInJSONL, "jsonl", false, "")
	pflag.StringVarP(&outputFilePath, "output", "o", "", "")
	pflag.StringVarP(&outputDirectoryPath, "output-directory", "O", "", "")
	pflag.BoolVarP(&monochrome, "monochrome", "m", false, "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.Usage = func() {
		hqgologger.Info(configuration.BANNER(au), hqgologger.WithLabel(""))

		h := "USAGE:\n"
		h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.NAME)

		h += "\nCONFIGURATION:\n"

		defaultConfigurationFilePath := strings.ReplaceAll(configuration.DefaultConfigurationFilePath, configuration.UserDotConfigDirectoryPath, "$HOME/.config")

		h += fmt.Sprintf(" -c, --configuration string           (default: %v)\n", au.Underline(defaultConfigurationFilePath).Bold())

		h += "\nINPUT:\n"
		h += " -d, --domain string[]                target domain\n"
		h += " -l, --list string                    target domains file path\n"

		h += "\n For multiple domains, use comma(,) separated value with `--domain`,\n"
		h += " specify multiple `--domains`, load from file with `--list` or load from stdin.\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool        match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "     --sources bool                   list supported sources\n"
		h += " -u, --sources-to-use string[]        comma(,) separated sources to use\n"
		h += " -e, --sources-to-exclude string[]    comma(,) separated sources to exclude\n"

		h += "\nOUTPUT:\n"
		h += "     --jsonl bool                     output in JSONL(ines)\n"
		h += " -o, --output string                  output write file path\n"
		h += " -O, --output-directory string        output write directory path\n"
		h += " -m, --monochrome bool                stdout in monochrome\n"
		h += " -s, --silent bool                    stdout in silent mode\n"
		h += " -v, --verbose bool                   stdout in verbose mode\n"

		hqgologger.Info(h, hqgologger.WithLabel(""))
		hqgologger.Print("")
	}

	pflag.Parse()

	if err := configuration.CreateUpdate(configurationFilePath); err != nil {
		hqgologger.Fatal("failed creating or updating Configuration!", hqgologger.WithError(err))
	}

	viper.SetConfigFile(configurationFilePath)
	viper.AutomaticEnv()
	viper.SetEnvPrefix(strings.ToUpper(configuration.NAME))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		hqgologger.Fatal("failed reading in Configuration!", hqgologger.WithError(err))
	}

	hqgologger.DefaultLogger.SetFormatter(hqgologgerformatter.NewConsoleFormatter(&hqgologgerformatter.ConsoleFormatterConfiguration{
		Colorize: !monochrome,
	}))

	if silent {
		hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelSilent)
	}

	if verbose {
		hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelDebug)
	}

	au = aurora.New(aurora.WithColors(!monochrome))
}

func main() {
	hqgologger.Info(configuration.BANNER(au), hqgologger.WithLabel(""))

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		hqgologger.Fatal("failed unmarshalling Configuration!", hqgologger.WithError(err))
	}

	if listSupportedSources {
		hqgologger.Info(fmt.Sprintf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(cfg.Sources))).Bold()))
		hqgologger.Info(fmt.Sprintf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold()))
		hqgologger.Print("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&cfg.Keys).Elem()

		for i := range keysElem.NumField() {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for index := range cfg.Sources {
			source := cfg.Sources[index]

			if _, ok := needsKey[source]; ok {
				hqgologger.Print("> " + source + " *")
			} else {
				hqgologger.Print("> " + source)
			}
		}

		hqgologger.Print("")

		os.Exit(0)
	}

	if domainsFilePath != "" {
		file, err := os.Open(domainsFilePath)
		if err != nil {
			hqgologger.Fatal("failed opening input file", hqgologger.WithError(err))
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				domains = append(domains, domain)
			}
		}

		if err := scanner.Err(); err != nil {
			hqgologger.Fatal("failed reading input file!", hqgologger.WithError(err))
		}

		file.Close()
	}

	if input.HasStdin() {
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			domain := scanner.Text()

			if domain != "" {
				domains = append(domains, domain)
			}
		}

		if err := scanner.Err(); err != nil {
			hqgologger.Fatal("failed reading stdin!", hqgologger.WithError(err))
		}
	}

	writer := output.NewWriter()

	if outputInJSONL {
		writer.SetFormatToJSONL()
	}

	finder, err := xurlfind3r.New(&xurlfind3r.Configuration{
		Client: &xurlfind3r.ClientConfiguration{
			UserAgent: fmt.Sprintf("%s %s (https://github.com/hueristiq/%s.git)", configuration.NAME, configuration.VERSION, configuration.NAME),
		},
		IncludeSubdomains: includeSubdomains,
		SourcesToUse:      sourcesToUse,
		SourcesToExclude:  sourcesToExclude,
		Keys:              cfg.Keys,
	})
	if err != nil {
		hqgologger.Fatal("failed creating finder!", hqgologger.WithError(err))
	}

	for index := range domains {
		domain := domains[index]

		hqgologger.Info(fmt.Sprintf("Finding URLs for %v...", au.Underline(domain).Bold()))
		hqgologger.Print("")

		outputs := []io.Writer{
			os.Stdout,
		}

		var file *os.File

		switch {
		case outputFilePath != "":
			file, err = writer.CreateFile(outputFilePath)
			if err != nil {
				hqgologger.Fatal("failed creating output file!", hqgologger.WithError(err), hqgologger.WithString("file", outputFilePath))
			}

			outputs = append(outputs, file)
		case outputDirectoryPath != "":
			file, err = writer.CreateFile(filepath.Join(outputDirectoryPath, domain))
			if err != nil {
				hqgologger.Fatal("failed creating output file!", hqgologger.WithError(err), hqgologger.WithString("file", outputFilePath))
			}

			outputs = append(outputs, file)
		}

		results := finder.Find(domain)

		for result := range results {
			for _, output := range outputs {
				switch result.Type {
				case sources.ResultError:
					if verbose {
						hqgologger.Error("error finding subdomains!", hqgologger.WithError(err), hqgologger.WithString("source", result.Source))
					}
				case sources.ResultURL:
					if err := writer.Write(output, domain, result); err != nil {
						hqgologger.Error("error writing subdomain!", hqgologger.WithError(err), hqgologger.WithString("source", result.Source))
					}
				}
			}
		}

		file.Close()

		hqgologger.Print("")
	}
}
