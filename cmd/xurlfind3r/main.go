package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	hqgologger "github.com/hueristiq/hq-lib-logger-go"
	hqgologgerformatter "github.com/hueristiq/hq-lib-logger-go/formatter"
	hqgologgerlevels "github.com/hueristiq/hq-lib-logger-go/levels"
	"github.com/logrusorgru/aurora/v4"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/internal/input"
	"github.com/hueristiq/xurlfind3r/internal/output"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
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
		hqgologger.Info(configuration.BANNER(au), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

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
		h += " -m, --monochrome bool                disable colored console output\n"
		h += " -s, --silent bool                    disable logging output, only results\n"
		h += " -v, --verbose bool                   enable detailed debug logging output\n"

		hqgologger.Info(h, hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
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

	hqgologger.DefaultLogger.SetFormatter(
		hqgologgerformatter.NewConsoleFormatter(&hqgologgerformatter.ConsoleFormatterConfiguration{
			Colorize: !monochrome,
		}),
	)

	if silent {
		// SetLevel can only fail on an unknown level; the levels used
		// here are constants, so the error is discarded deliberately.
		_ = hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelSilent)
	}

	if verbose {
		_ = hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelDebug)
	}

	au = aurora.New(aurora.WithColors(!monochrome))
}

func main() {
	hqgologger.Info(configuration.BANNER(au), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		hqgologger.Fatal("failed unmarshalling Configuration!", hqgologger.WithError(err))
	}

	if listSupportedSources {
		hqgologger.Info(fmt.Sprintf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(xurlfind3r.Sources))).Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Info(fmt.Sprintf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		for i := range xurlfind3r.Sources {
			source := xurlfind3r.Sources[i]

			if source.NeedsKeys() {
				hqgologger.Print("> "+source.Name()+" *", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
			} else {
				hqgologger.Print("> "+source.Name(), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
			}
		}

		hqgologger.Print("")

		os.Exit(0)
	}

	if domainsFilePath != "" {
		//nolint:gosec // file path is user-provided via the --list flag; opening it is the intended behavior.
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

		if err := file.Close(); err != nil {
			hqgologger.Fatal("failed closing input file!", hqgologger.WithError(err))
		}
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	finder, err := xurlfind3r.New(&xurlfind3r.Configuration{
		Client: &xurlfind3r.ClientConfiguration{
			UserAgent: fmt.Sprintf("%s %s (https://github.com/hueristiq/%s.git)", configuration.NAME, configuration.Version, configuration.NAME),
		},
		SourcesToUse:      sourcesToUse,
		SourcesToExclude:  sourcesToExclude,
		Keys:              cfg.Keys,
		IncludeSubdomains: includeSubdomains,
	})
	if err != nil {
		hqgologger.Fatal("failed creating finder!", hqgologger.WithError(err))
	}

	for index := range domains {
		domain := domains[index]

		hqgologger.Info(fmt.Sprintf("Finding URLs for %v...", au.Underline(domain).Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		outputs := []io.Writer{
			os.Stdout,
		}

		var file *os.File

		switch {
		case outputFilePath != "":
			file, err = writer.CreateFile(outputFilePath)
			if err != nil {
				hqgologger.Fatal("failed to create output file!", hqgologger.WithString("file", outputFilePath), hqgologger.WithError(err))
			}

			outputs = append(outputs, file)
		case outputDirectoryPath != "":
			domainOutputFilePath := filepath.Join(outputDirectoryPath, domain)

			file, err = writer.CreateFile(domainOutputFilePath)
			if err != nil {
				hqgologger.Fatal("failed to create output file!", hqgologger.WithString("file", domainOutputFilePath), hqgologger.WithError(err))
			}

			outputs = append(outputs, file)
		}

		results := finder.Find(ctx, domain)

		for result := range results {
			for _, output := range outputs {
				switch result.Type {
				case sources.ResultError:
					if verbose {
						hqgologger.Error("failed to find results!", hqgologger.WithString("source", result.Source), hqgologger.WithError(result.Error))
					}
				case sources.ResultURL:
					if err := writer.Write(output, domain, result); err != nil {
						hqgologger.Error("failed to write result!", hqgologger.WithString("source", result.Source), hqgologger.WithError(err))
					}
				}
			}
		}

		if file != nil {
			if err := file.Close(); err != nil {
				hqgologger.Fatal("failed closing output file!", hqgologger.WithString("file", file.Name()), hqgologger.WithError(err))
			}
		}

		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
	}
}
