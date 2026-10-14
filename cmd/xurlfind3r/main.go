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
	"syscall"

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
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.DefaultConfigurationFilePath(), "")
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
		hqgologger.Info(configuration.Banner(au), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		h := "USAGE:\n"
		h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.Name)

		h += "\nCONFIGURATION:\n"

		defaultConfigurationFilePath := configuration.DefaultConfigurationFilePath()

		if userConfigDirectoryPath, err := configuration.UserConfigDirectoryPath(); err == nil {
			defaultConfigurationFilePath = strings.ReplaceAll(defaultConfigurationFilePath, userConfigDirectoryPath, "$HOME/.config")
		}

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

	if err := configuration.CreateOrUpdate(configurationFilePath); err != nil {
		hqgologger.Fatal("failed creating or updating Configuration!", hqgologger.WithError(err))
	}

	viper.SetConfigFile(configurationFilePath)

	viper.AutomaticEnv()

	viper.SetEnvPrefix(strings.ToUpper(configuration.Name))
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
		if err := hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelSilent); err != nil {
			hqgologger.Fatal("failed setting logger level!", hqgologger.WithError(err))
		}
	}

	if verbose {
		if err := hqgologger.DefaultLogger.SetLevel(hqgologgerlevels.LevelDebug); err != nil {
			hqgologger.Fatal("failed setting logger level!", hqgologger.WithError(err))
		}
	}

	au = aurora.New(aurora.WithColors(!monochrome))
}

func main() {
	hqgologger.Info(configuration.Banner(au), hqgologger.WithoutLabel())

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		hqgologger.Fatal("failed unmarshalling Configuration!", hqgologger.WithError(err))
	}

	if listSupportedSources {
		hqgologger.Info(fmt.Sprintf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(sources.List))).Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Info(fmt.Sprintf("sources marked with %v take in key(s) or token(s).", au.Underline("*").Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		for i := range sources.List {
			name := sources.List[i]

			source, ok := xurlfind3r.NewSource(name)
			if !ok {
				continue
			}

			if source.NeedsKeys() {
				hqgologger.Print("> "+name+" *", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
			} else {
				hqgologger.Print("> "+name, hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
			}
		}

		hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		os.Exit(0)
	}

	if domainsFilePath != "" {
		file, err := os.Open(domainsFilePath) //nolint:gosec // path is the user-specified domains file location.
		if err != nil {
			hqgologger.Fatal("failed opening input file", hqgologger.WithError(err))
		}

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			domains = append(domains, scanner.Text())
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
			domains = append(domains, scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			hqgologger.Fatal("failed reading stdin!", hqgologger.WithError(err))
		}
	}

	validatedDomains := make([]string, 0, len(domains))

	for _, domain := range domains {
		domain = strings.TrimSpace(domain)

		if domain == "" {
			continue
		}

		if err := input.ValidateDomain(domain); err != nil {
			hqgologger.Fatal("invalid domain!", hqgologger.WithString("domain", domain), hqgologger.WithError(err))
		}

		validatedDomains = append(validatedDomains, domain)
	}

	domains = validatedDomains

	if len(domains) == 0 {
		hqgologger.Fatal("no domains provided! specify at least one with --domain, --list or stdin.")
	}

	writer := output.NewWriter()

	if outputInJSONL {
		writer.SetFormatToJSONL()
	}

	signal.Ignore(syscall.SIGPIPE)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer stop()

	if len(sourcesToUse) == 0 {
		sourcesToUse = cfg.Sources
	}

	finder, err := xurlfind3r.New(&xurlfind3r.Configuration{
		Client: &xurlfind3r.ClientConfiguration{
			UserAgent: fmt.Sprintf("%s %s (https://github.com/hueristiq/%s.git)", configuration.Name, configuration.Version, configuration.Name),
		},
		SourcesToUse:      sourcesToUse,
		SourcesToExclude:  sourcesToExclude,
		Keys:              cfg.Keys,
		IncludeSubdomains: includeSubdomains,
	})
	if err != nil {
		hqgologger.Fatal("failed creating finder!", hqgologger.WithError(err))
	}

	outputs := []io.Writer{
		os.Stdout,
	}

	var file *os.File

	if outputFilePath != "" {
		file, err = writer.CreateFile(outputFilePath)
		if err != nil {
			hqgologger.Fatal("failed to create output file!", hqgologger.WithString("file", outputFilePath), hqgologger.WithError(err))
		}

		outputs = append(outputs, file)
	}

	for index := range domains {
		domain := domains[index]

		hqgologger.Info(fmt.Sprintf("Finding URLs for %v...", au.Underline(domain).Bold()), hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

		if !silent {
			hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		}

		domainOutputs := outputs

		var domainFile *os.File

		if outputFilePath == "" && outputDirectoryPath != "" {
			domainOutputFilePath := filepath.Join(outputDirectoryPath, domain)

			domainFile, err = writer.CreateFile(domainOutputFilePath)
			if err != nil {
				hqgologger.Fatal("failed to create output file!", hqgologger.WithString("file", domainOutputFilePath), hqgologger.WithError(err))
			}

			domainOutputs = append(domainOutputs, domainFile)
		}

		results := finder.Find(ctx, domain)

		for result := range results {
			for _, output := range domainOutputs {
				switch result.Type {
				case sources.ResultError:
					if verbose {
						hqgologger.Error("failed to find results!", hqgologger.WithString("source", result.Source), hqgologger.WithError(result.Error))
					}
				case sources.ResultURL:
					if err := writer.Write(output, domain, result); err != nil {
						if output != os.Stdout {
							_ = writer.Flush(os.Stdout)

							hqgologger.Fatal("failed to write result to output file!", hqgologger.WithError(err))
						}

						domainOutputs = dropOutput(domainOutputs, output)

						hqgologger.Error("failed to write result to stdout; dropping stdout output!", hqgologger.WithError(err))

						if len(domainOutputs) == 0 {
							stop()
						}
					}
				}
			}

			for _, output := range domainOutputs {
				if err := writer.Flush(output); err != nil {
					if output != os.Stdout {
						_ = writer.Flush(os.Stdout)

						hqgologger.Fatal("failed to flush output file!", hqgologger.WithError(err))
					}

					domainOutputs = dropOutput(domainOutputs, output)

					hqgologger.Error("failed to flush stdout; dropping stdout output!", hqgologger.WithError(err))

					if len(domainOutputs) == 0 {
						stop()
					}
				}
			}
		}

		if domainFile != nil {
			if err := domainFile.Close(); err != nil {
				hqgologger.Fatal("failed closing output file!", hqgologger.WithError(err))
			}
		}

		if !silent {
			hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		}
	}

	if file != nil {
		if err := file.Close(); err != nil {
			hqgologger.Fatal("failed closing output file!", hqgologger.WithError(err))
		}
	}
}

func dropOutput(outputs []io.Writer, dropped io.Writer) (remaining []io.Writer) {
	remaining = make([]io.Writer, 0, len(outputs))

	for _, output := range outputs {
		if output != dropped {
			remaining = append(remaining, output)
		}
	}

	return
}
