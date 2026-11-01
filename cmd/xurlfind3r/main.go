package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	hqgologger "github.com/hueristiq/hq-lib-logger-go"
	hqgologgerformatter "github.com/hueristiq/hq-lib-logger-go/formatter"
	hqgologgerlevel "github.com/hueristiq/hq-lib-logger-go/level"
	hqgologgerwriter "github.com/hueristiq/hq-lib-logger-go/writer"
	hqgotuiansi "github.com/hueristiq/hq-lib-tui-go/ansi"
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

var logger = slog.New(hqgologger.New().SlogHandler())

var profile = hqgotuiansi.DetectProfile(os.Stderr)

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
		fatal(logger, "failed creating or updating Configuration!", slog.Any("error", err))
	}

	viper.SetConfigFile(configurationFilePath)

	viper.AutomaticEnv()

	viper.SetEnvPrefix(strings.ToUpper(configuration.Name))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		fatal(logger, "failed reading in Configuration!", slog.Any("error", err))
	}

	level := hqgologgerlevel.LevelInfo

	if silent {
		level = hqgologgerlevel.LevelFatal
	}

	if verbose {
		level = hqgologgerlevel.LevelDebug
	}

	logger = slog.New(hqgologger.New(
		hqgologger.WithFormatter(hqgologgerformatter.NewConsole(&hqgologgerformatter.ConsoleConfig{
			Label:    true,
			Colorize: !monochrome,
			Colorizer: hqgologgerformatter.ColorizerFunc(func(text string, lvl hqgologgerlevel.Level) string {
				return levelStyle(lvl).Render(profile, text)
			}),
		})),
		hqgologger.WithWriter(hqgologgerwriter.NewStream(os.Stderr)),
		hqgologger.WithLevel(level),
	).SlogHandler())
}

func usage() {
	fmt.Fprintln(os.Stderr, configuration.Banner)

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

	fmt.Fprint(os.Stderr, h)
	fmt.Fprintln(os.Stdout) //nolint:errcheck // blank separator write errors are non-fatal.
}

func main() {
	configure()

	if !silent {
		fmt.Fprintln(os.Stderr, configuration.Banner)
	}

	var cfg *configuration.Configuration

	if err := viper.Unmarshal(&cfg); err != nil {
		fatal(logger, "failed unmarshalling Configuration!", slog.Any("error", err))
	}

	if listSupportedSources {
		printSources()

		return
	}

	collectDomainsFromFile()
	collectDomainsFromStdin()

	domains = normalizeDomains(domains)

	if len(domains) == 0 {
		fatal(logger, "no domains provided! specify at least one with --domain, --list or stdin.")
	}

	writer := output.NewWriter()

	if outputInJSONL {
		writer.SetFormatToJSONL()
	}

	if !monochrome {
		writer.SetSourceProfile(hqgotuiansi.DetectProfile(os.Stdout))
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
		fatal(logger, "failed creating finder!", slog.Any("error", err))
	}

	outputs := []io.Writer{
		os.Stdout,
	}

	var file *os.File

	if outputFilePath != "" {
		file, err = writer.CreateFile(outputFilePath)
		if err != nil {
			fatal(logger, "failed to create output file!", slog.String("file", outputFilePath), slog.Any("error", err))
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

		logger.Info(fmt.Sprintf("Finding URLs for %v...", domain))

		if !silent {
			fmt.Fprintln(os.Stdout) //nolint:errcheck // blank separator write errors are non-fatal.
		}

		domainOutputs := outputs

		var domainFile *os.File

		if outputFilePath == "" && outputDirectoryPath != "" {
			domainOutputFilePath := filepath.Join(outputDirectoryPath, domain)

			domainFile, err = writer.CreateFile(domainOutputFilePath)
			if err != nil {
				fatal(logger, "failed to create output file!", slog.String("file", domainOutputFilePath), slog.Any("error", err))
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
				fatal(logger, "failed flushing output file!", slog.Any("error", err))
			}

			if err := domainFile.Close(); err != nil {
				fatal(logger, "failed closing output file!", slog.Any("error", err))
			}
		}

		if !silent {
			fmt.Fprintln(os.Stdout) //nolint:errcheck // blank separator write errors are non-fatal.
		}
	}

	if file != nil {
		if err := writer.Release(file); err != nil {
			fatal(logger, "failed flushing output file!", slog.Any("error", err))
		}

		if err := file.Close(); err != nil {
			fatal(logger, "failed closing output file!", slog.Any("error", err))
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
	logger.Info(fmt.Sprintf("listing, %v, current supported sources.", strconv.Itoa(len(sources.List))))
	logger.Info(fmt.Sprintf("sources marked with %v take in key(s) or token(s).", "*"))
	fmt.Fprintln(os.Stdout) //nolint:errcheck // blank separator write errors are non-fatal.

	for i := range sources.List {
		name := sources.List[i]

		source, ok := xurlfind3r.NewSource(name)
		if !ok {
			continue
		}

		if source.NeedsKeys() {
			fmt.Fprintln(os.Stdout, "> "+name+" *") //nolint:errcheck // source listing write errors are non-fatal.
		} else {
			fmt.Fprintln(os.Stdout, "> "+name) //nolint:errcheck // source listing write errors are non-fatal.
		}
	}

	fmt.Fprintln(os.Stdout) //nolint:errcheck // blank separator write errors are non-fatal.
}

func collectDomainsFromFile() {
	if domainsFilePath == "" {
		return
	}

	file, err := os.Open(domainsFilePath) //nolint:gosec // path is the user-specified domains file location.
	if err != nil {
		fatal(logger, "failed opening input file", slog.Any("error", err))
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		domains = append(domains, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fatal(logger, "failed reading input file!", slog.Any("error", err))
	}

	if err := file.Close(); err != nil {
		fatal(logger, "failed closing input file!", slog.Any("error", err))
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
		fatal(logger, "failed reading stdin!", slog.Any("error", err))
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
			fatal(logger, "invalid domain!", slog.String("domain", domain), slog.Any("error", err))
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
		logger.Error(result.Error.Error(), sourceField(result.Source))
	}

	if result.Type == sources.ResultWarning {
		logger.Warn(result.Value, sourceField(result.Source))
	}
}

func sourceField(source string) (field slog.Attr) {
	value := source

	if !monochrome {
		value = hqgotuiansi.Style{Fg: hqgotuiansi.BrightCyan}.Render(profile, source)
	}

	field = slog.String("source", value)

	return
}

func writeResult(writer *output.Writer, outputs []io.Writer, domain string, result sources.Result, silentMode bool) (remaining []io.Writer) {
	remaining = outputs

	for _, destination := range outputs {
		write := writer.Write

		if destination == os.Stdout && verbose && !silentMode {
			write = writer.WriteWithSource
		}

		if err := write(destination, domain, result); err != nil {
			if destination != os.Stdout {
				_ = writer.Flush(os.Stdout)

				fatal(logger, "failed to write result to output file!", slog.Any("error", err))
			}

			logger.Error("failed to write result to stdout; dropping stdout output!", slog.Any("error", err))

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

				fatal(logger, "failed to flush output file!", slog.Any("error", err))
			}

			logger.Error("failed to flush stdout; dropping stdout output!", slog.Any("error", err))

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

func levelStyle(level hqgologgerlevel.Level) (style hqgotuiansi.Style) {
	switch level {
	case hqgologgerlevel.LevelFatal, hqgologgerlevel.LevelError:
		style = hqgotuiansi.Style{Fg: hqgotuiansi.BrightRed, Attributes: hqgotuiansi.Bold}
	case hqgologgerlevel.LevelWarn:
		style = hqgotuiansi.Style{Fg: hqgotuiansi.BrightYellow, Attributes: hqgotuiansi.Bold}
	case hqgologgerlevel.LevelInfo:
		style = hqgotuiansi.Style{Fg: hqgotuiansi.BrightBlue, Attributes: hqgotuiansi.Bold}
	case hqgologgerlevel.LevelDebug:
		style = hqgotuiansi.Style{Fg: hqgotuiansi.BrightMagenta, Attributes: hqgotuiansi.Bold}
	case hqgologgerlevel.LevelTrace:
		style = hqgotuiansi.Style{Attributes: hqgotuiansi.Faint}
	}

	return
}
