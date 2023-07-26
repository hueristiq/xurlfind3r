package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/hueristiq/hqgolog"
	"github.com/hueristiq/hqgolog/formatter"
	"github.com/hueristiq/hqgolog/levels"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
)

var (
	au aurora.Aurora

	domainsSlice        []string
	domainsListFilePath string
	includeSubdomains   bool
	listSources         bool
	sourcesToUse        []string
	sourcesToExclude    []string
	parseWaybackRobots  bool
	parseWaybackSource  bool
	threads             int
	filterPattern       string
	matchPattern        string
	monochrome          bool
	output              string
	outputDirectory     string
	verbosity           string
	YAMLConfigFile      string
)

func init() {
	// defaults
	defaultThreads := 50
	defaultYAMLConfigFile := fmt.Sprintf("~/.hueristiq/%s/config.yaml", configuration.NAME)

	// Handle CLI arguments, flags & help message (pflag)
	pflag.StringSliceVarP(&domainsSlice, "domain", "d", []string{}, "")
	pflag.StringVarP(&domainsListFilePath, "list", "l", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSources, "sources", false, "")
	pflag.StringSliceVarP(&sourcesToUse, "use-sources", "u", []string{}, "")
	pflag.StringSliceVarP(&sourcesToExclude, "exclude-sources", "e", []string{}, "")
	pflag.BoolVar(&parseWaybackRobots, "parse-wayback-robots", false, "")
	pflag.BoolVar(&parseWaybackSource, "parse-wayback-source", false, "")
	pflag.IntVarP(&threads, "threads", "t", defaultThreads, "")
	pflag.StringVarP(&filterPattern, "filter", "f", "", "")
	pflag.StringVarP(&matchPattern, "match", "m", "", "")
	pflag.BoolVar(&monochrome, "no-color", false, "")
	pflag.StringVarP(&output, "output", "o", "", "")
	pflag.StringVarP(&outputDirectory, "outputDirectory", "O", "", "")
	pflag.StringVarP(&verbosity, "verbosity", "v", string(levels.LevelInfo), "")
	pflag.StringVarP(&YAMLConfigFile, "configuration", "c", defaultYAMLConfigFile, "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "USAGE:\n"
		h += "  xurlfind3r [OPTIONS]\n"

		h += "\nINPUT:\n"
		h += " -d, --domain string[]               target domains\n"
		h += " -l, --list string                   target domains' list file path\n"

		h += "\nSCOPE:\n"
		h += "     --include-subdomains bool       match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += "      --sources bool                 list supported sources\n"
		h += " -u,  --use-sources string[]         comma(,) separated sources to use\n"
		h += " -e,  --exclude-sources string[]     comma(,) separated sources to exclude\n"
		h += "      --parse-wayback-robots bool    with wayback, parse robots.txt snapshots\n"
		h += "      --parse-wayback-source bool    with wayback, parse source code snapshots\n"

		h += "\nOPTIMIZATION:\n"
		h += fmt.Sprintf(" -t,  --threads int                    number of threads (default: %d)\n", defaultThreads)

		h += "\nFILTER & MATCH:\n"
		h += " -f, --filter string                 regex to filter URLs\n"
		h += " -m, --match string                  regex to match URLs\n"

		h += "\nOUTPUT:\n"
		h += "     --no-color bool                 disable colored output\n"
		h += " -o, --output string                 output URLs file path\n"
		h += " -O, --output-directory string       output URLs directory path\n"
		h += fmt.Sprintf(" -v, --verbosity string              debug, info, warning, error, fatal or silent (default: %s)\n", string(levels.LevelInfo))

		h += "\nCONFIGURATION:\n"
		h += fmt.Sprintf(" -c,  --configuration string         configuration file path (default: %s)\n", defaultYAMLConfigFile)

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	// Initialize logger (hqgolog)
	hqgolog.DefaultLogger.SetMaxLevel(levels.LevelStr(verbosity))
	hqgolog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	// Create or Update configuration
	if strings.HasPrefix(YAMLConfigFile, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		YAMLConfigFile = strings.Replace(YAMLConfigFile, "~", home, 1)
	}

	if err := configuration.CreateUpdate(YAMLConfigFile); err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	au = aurora.NewAurora(!monochrome)
}

func main() {
	// Print Banner
	if verbosity != string(levels.LevelSilent) {
		fmt.Fprintln(os.Stderr, configuration.BANNER)
	}

	// Read in configuration
	config, err := configuration.Read(YAMLConfigFile)
	if err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	// List suported sources
	if listSources {
		hqgolog.Info().Msgf("listing, %v, current supported sources.", au.Underline(strconv.Itoa(len(config.Sources))).Bold())
		hqgolog.Info().Msgf("sources marked with %v need key(s) or token(s) to work.", au.Underline("*").Bold())
		hqgolog.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&config.Keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range config.Sources {
			if _, ok := needsKey[source]; ok {
				hqgolog.Print().Msgf("> %s *", source)
			} else {
				hqgolog.Print().Msgf("> %s", source)
			}
		}

		hqgolog.Print().Msg("")

		os.Exit(0)
	}

	domains := make(chan string, threads)

	// Load input domains
	go func() {
		defer close(domains)

		// input domains: slice
		for _, domain := range domainsSlice {
			domains <- domain
		}

		// input domains: file
		if domainsListFilePath != "" {
			file, err := os.Open(domainsListFilePath)
			if err != nil {
				hqgolog.Error().Msg(err.Error())
			}

			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				domain := scanner.Text()

				if domain != "" {
					domains <- domain
				}
			}

			if err := scanner.Err(); err != nil {
				hqgolog.Error().Msg(err.Error())
			}
		}

		// input domains: stdin
		if hasStdin() {
			scanner := bufio.NewScanner(os.Stdin)

			for scanner.Scan() {
				domain := scanner.Text()

				if domain != "" {
					domains <- domain
				}
			}

			if err := scanner.Err(); err != nil {
				hqgolog.Error().Msg(err.Error())
			}
		}
	}()

	// Find and output URLs.
	var consolidatedWriter *bufio.Writer

	if output != "" {
		directory := filepath.Dir(output)

		mkdir(directory)

		consolidatedFile, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		defer consolidatedFile.Close()

		consolidatedWriter = bufio.NewWriter(consolidatedFile)
	}

	if outputDirectory != "" {
		mkdir(outputDirectory)
	}

	wg := &sync.WaitGroup{}

	for i := 0; i < threads; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			options := &xurlfind3r.Options{
				IncludeSubdomains:  includeSubdomains,
				SourcesToUSe:       sourcesToUse,
				SourcesToExclude:   sourcesToExclude,
				Keys:               config.Keys,
				ParseWaybackRobots: parseWaybackRobots,
				ParseWaybackSource: parseWaybackSource,
				FilterPattern:      filterPattern,
				Matchattern:        matchPattern,
			}

			finder, err := xurlfind3r.New(options)
			if err != nil {
				hqgolog.Error().Msg(err.Error())

				return
			}

			for domain := range domains {
				URLs := finder.Find(domain)

				switch {
				case output != "":
					processURLs(consolidatedWriter, URLs, verbosity)
				case outputDirectory != "":
					domainFile, err := os.OpenFile(filepath.Join(outputDirectory, domain+".txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						hqgolog.Fatal().Msg(err.Error())
					}

					domainWriter := bufio.NewWriter(domainFile)

					processURLs(domainWriter, URLs, verbosity)
				default:
					processURLs(nil, URLs, verbosity)
				}
			}
		}()
	}

	wg.Wait()
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

func processURLs(writer *bufio.Writer, URLs chan sources.URL, verbosity string) {
	for URL := range URLs {
		if verbosity == string(levels.LevelSilent) {
			hqgolog.Print().Msg(URL.Value)
		} else {
			hqgolog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
		}

		if writer != nil {
			fmt.Fprintln(writer, URL.Value)

			if err := writer.Flush(); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}
		}
	}
}
