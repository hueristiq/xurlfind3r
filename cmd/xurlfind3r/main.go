package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	hqgologger "github.com/hueristiq/hq-lib-logger-go"
	hqgologgerformatter "github.com/hueristiq/hq-lib-logger-go/formatter"
	hqgologgerlevels "github.com/hueristiq/hq-lib-logger-go/levels"
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
)

func init() {
	pflag.StringVarP(&configurationFilePath, "configuration", "c", configuration.DefaultConfigurationFilePath(), "")
	pflag.StringSliceVarP(&domains, "domain", "d", []string{}, "")
	pflag.StringVarP(&domainsFilePath, "list", "l", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSupportedSources, "sources", false, "")
	pflag.StringSliceVar(&sourcesToUse, "sources-to-use", []string{}, "")
	pflag.StringSliceVar(&sourcesToExclude, "sources-to-exclude", []string{}, "")
	pflag.BoolVar(&outputInJSONL, "jsonl", false, "")
	pflag.StringVarP(&outputFilePath, "output", "o", "", "")
	pflag.StringVarP(&outputDirectoryPath, "output-directory", "O", "", "")
	pflag.BoolVarP(&monochrome, "monochrome", "m", false, "")
	pflag.BoolVarP(&silent, "silent", "s", false, "")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "")

	pflag.Usage = usage
}

func configure() {
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
			Colorize:     !monochrome,
			IncludeLabel: true,
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
}

func usage() {
	hqgologger.Info(configuration.Banner, hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())

	h := "USAGE:\n"
	h += fmt.Sprintf(" %s [OPTIONS]\n", configuration.Name)

	h += "\nCONFIGURATION:\n"

	defaultConfigurationFilePath := configuration.DefaultConfigurationFilePath()

	if userConfigDirectoryPath, err := configuration.UserConfigDirectoryPath(); err == nil {
		defaultConfigurationFilePath = strings.ReplaceAll(defaultConfigurationFilePath, userConfigDirectoryPath, "$HOME/.config")
	}

	h += fmt.Sprintf(" -c, --configuration string           (default: %v)\n", defaultConfigurationFilePath)

	h += "\nINPUT:\n"
	h += " -d, --domain string[]                target domain\n"
	h += " -l, --list string                    target domains file path\n"

	h += "\n For multiple domains, use comma(,) separated value with `--domain`,\n"
	h += " specify multiple `--domain`, load from file with `--list` or load from stdin.\n"

	h += "\nSCOPE:\n"
	h += "     --include-subdomains bool        match subdomain's URLs\n"

	h += "\nSOURCES:\n"
	h += "     --sources bool                   list supported sources\n"
	h += "     --sources-to-use string[]        comma(,) separated sources to use\n"
	h += "     --sources-to-exclude string[]    comma(,) separated sources to exclude\n"

	h += "\nOUTPUT:\n"
	h += "     --jsonl bool                     output in JSONL(ines)\n"
	h += " -o, --output string                  output write file path\n"
	h += " -O, --output-directory string        output write directory path\n"
	h += " -m, --monochrome bool                stdout in monochrome\n"
	h += " -s, --silent bool                    stdout in silent mode\n"
	h += " -v, --verbose bool                   stdout in verbose mode\n"

	hqgologger.Info(h, hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
	hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
}

func main() {
	configure()

	hqgologger.Info(configuration.Banner, hqgologger.WithoutLabel())

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		hqgologger.Fatal("failed unmarshalling Configuration!", hqgologger.WithError(err))
	}

	if listSupportedSources {
		printSources()

		return
	}

	collectDomainsFromFile()
	collectDomainsFromStdin()

	domains = normalizeDomains(domains)

	if len(domains) == 0 {
		hqgologger.Fatal("no domains provided! specify at least one with --domain, --list or stdin.")
	}

	writer := output.NewWriter()

	if outputInJSONL {
		writer.SetFormatToJSONL()
	}

	ignoreBrokenPipe()

	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)

	go func() {
		<-ctx.Done()

		stop()
	}()

	if len(sourcesToUse) == 0 {
		sourcesToUse = cfg.Sources
	}

	finder, err := xurlfind3r.New(&xurlfind3r.Configuration{
		Client: &xurlfind3r.ClientConfiguration{
			UserAgent: fmt.Sprintf("%s %s", configuration.Name, configuration.Version),
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

	sourceCount := finder.SourceCount()
	urlCount := 0
	finderFailed := false
	failedSources := make(map[string]struct{})

	for index := range domains {
		domain := domains[index]

		if ctx.Err() != nil {
			break
		}

		hqgologger.Info(fmt.Sprintf("Finding URLs for %v...", domain), hqgologger.WithoutTimestamp())

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
			if ctx.Err() != nil {
				break
			}

			logResult(result)

			if result.Type == sources.ResultError {
				if result.Source == configuration.Name {
					finderFailed = true
				} else {
					failedSources[result.Source] = struct{}{}
				}
			}

			if result.Type == sources.ResultURL {
				urlCount++

				domainOutputs = writeResult(writer, domainOutputs, domain, result, silent)
				domainOutputs = flushOutputs(writer, domainOutputs)

				if len(domainOutputs) == 0 {
					stop()
				}
			}
		}

		if domainFile != nil {
			if err := writer.Release(domainFile); err != nil {
				hqgologger.Fatal("failed flushing output file!", hqgologger.WithError(err))
			}

			if err := domainFile.Close(); err != nil {
				hqgologger.Fatal("failed closing output file!", hqgologger.WithError(err))
			}
		}

		if !silent {
			hqgologger.Print("", hqgologger.WithoutTimestamp(), hqgologger.WithoutLabel())
		}
	}

	if file != nil {
		if err := writer.Release(file); err != nil {
			hqgologger.Fatal("failed flushing output file!", hqgologger.WithError(err))
		}

		if err := file.Close(); err != nil {
			hqgologger.Fatal("failed closing output file!", hqgologger.WithError(err))
		}
	}

	if ctx.Err() != nil {
		stop()

		os.Exit(130)
	}

	if finderFailed || (urlCount == 0 && sourceCount > 0 && len(failedSources) >= sourceCount) {
		stop()

		os.Exit(1)
	}

	stop()
}

func printSources() {
	hqgologger.Info(fmt.Sprintf("listing, %v, current supported sources.", strconv.Itoa(len(sources.List))), hqgologger.WithoutTimestamp())
	hqgologger.Info(fmt.Sprintf("sources marked with %v take in key(s) or token(s).", "*"), hqgologger.WithoutTimestamp())
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
}

func collectDomainsFromFile() {
	if domainsFilePath == "" {
		return
	}

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

func collectDomainsFromStdin() {
	if !input.HasStdin() {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		domains = append(domains, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		hqgologger.Fatal("failed reading stdin!", hqgologger.WithError(err))
	}
}

func normalizeDomains(rawDomains []string) (normalized []string) {
	normalized = make([]string, 0, len(rawDomains))
	seen := make(map[string]struct{}, len(rawDomains))

	for _, domain := range rawDomains {
		domain = strings.TrimSpace(domain)

		if domain == "" {
			continue
		}

		normalizedDomain, err := input.NormalizeDomain(domain)
		if err != nil {
			hqgologger.Fatal("invalid domain!", hqgologger.WithString("domain", domain), hqgologger.WithError(err))
		}

		if _, exists := seen[normalizedDomain]; exists {
			continue
		}

		seen[normalizedDomain] = struct{}{}

		normalized = append(normalized, normalizedDomain)
	}

	return
}

func logResult(result sources.Result) {
	if result.Type == sources.ResultError && verbose && result.Error != nil {
		hqgologger.Error(result.Error.Error(), hqgologger.WithString("source", result.Source))
	}

	if result.Type == sources.ResultWarning {
		hqgologger.Warn(result.Value, hqgologger.WithString("source", result.Source))
	}
}

func writeResult(writer *output.Writer, outputs []io.Writer, domain string, result sources.Result, silentMode bool) (remaining []io.Writer) {
	remaining = outputs

	for _, destination := range outputs {
		write := writer.Write

		if destination == os.Stdout && !silentMode {
			write = writer.WriteWithSource
		}

		if err := write(destination, domain, result); err != nil {
			if destination != os.Stdout {
				_ = writer.Flush(os.Stdout)

				hqgologger.Fatal("failed to write result to output file!", hqgologger.WithError(err))
			}

			hqgologger.Error("failed to write result to stdout; dropping stdout output!", hqgologger.WithError(err))

			remaining = dropOutput(remaining, destination)
		}
	}

	return
}

func flushOutputs(writer *output.Writer, outputs []io.Writer) (remaining []io.Writer) {
	remaining = outputs

	for _, destination := range outputs {
		if err := writer.Flush(destination); err != nil {
			if destination != os.Stdout {
				_ = writer.Flush(os.Stdout)

				hqgologger.Fatal("failed to flush output file!", hqgologger.WithError(err))
			}

			hqgologger.Error("failed to flush stdout; dropping stdout output!", hqgologger.WithError(err))

			remaining = dropOutput(remaining, destination)
		}
	}

	return
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

func ignoreBrokenPipe() {
	if runtime.GOOS == "windows" {
		return
	}

	signal.Ignore(syscall.SIGPIPE)
}

func terminationSignals() (signals []os.Signal) {
	if runtime.GOOS == "windows" {
		signals = []os.Signal{os.Interrupt}

		return
	}

	signals = []os.Signal{os.Interrupt, syscall.SIGTERM}

	return
}
