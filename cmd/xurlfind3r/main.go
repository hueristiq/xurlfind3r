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
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
)

var (
	au aurora.Aurora

	YAMLConfigFile string

	domain            string
	includeSubdomains bool

	listSources       bool
	sourcesToUse      []string
	skipWaybackRobots bool
	skipWaybackSource bool

	filterPattern string
	matchPattern  string

	monochrome bool
	output     string
	verbosity  string
)

func init() {
	// defaults
	defaultYAMLConfigFile := "~/.huristiq/xurlfind3r/config.yaml"

	// Handle CLI arguments, flags & help message (pflag)
	pflag.StringVarP(&YAMLConfigFile, "configuration", "c", defaultYAMLConfigFile, "")

	pflag.StringVarP(&domain, "domain", "d", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")

	pflag.BoolVarP(&listSources, "sources", "s", false, "")
	pflag.StringSliceVarP(&sourcesToUse, "use", "u", sources.List, "")
	pflag.BoolVar(&skipWaybackRobots, "skip-wayback-robots", false, "")
	pflag.BoolVar(&skipWaybackSource, "skip-wayback-source", false, "")

	pflag.StringVarP(&filterPattern, "filter", "f", "", "")
	pflag.StringVarP(&matchPattern, "match", "m", "", "")

	pflag.BoolVar(&monochrome, "no-color", false, "")
	pflag.StringVarP(&output, "output", "o", "", "")
	pflag.StringVarP(&verbosity, "verbosity", "v", string(levels.LevelInfo), "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "USAGE:\n"
		h += "  xurlfind3r [OPTIONS]\n"

		h += "\nCONFIGURATION:\n"
		h += fmt.Sprintf(" -c   --configuration string      configuration file path (default: %s)\n", defaultYAMLConfigFile)

		h += "\nSCOPE:\n"
		h += "  -d, --domain string             (sub)domain to match URLs\n"
		h += "      --include-subdomains bool   match subdomain's URLs\n"

		h += "\nSOURCES:\n"
		h += " -s,  --sources bool              list sources\n"
		h += " -u   --use string                sources to use (default: commoncrawl,github,intelx,otx,urlscan,wayback)\n"
		h += "      --skip-wayback-robots bool  with wayback, skip parsing robots.txt snapshots\n"
		h += "      --skip-wayback-source bool  with wayback, skip parsing source code snapshots\n"

		h += "\nFILTER & MATCH:\n"
		h += "  -f, --filter string             regex to filter URLs\n"
		h += "  -m, --match string              regex to match URLs\n"

		h += "\nOUTPUT:\n"
		h += "      --no-color                  no color mode\n"
		h += "  -o, --output string             output URLs file path\n"
		h += fmt.Sprintf("  -v, --verbosity                 debug, info, warning, error, fatal or silent (default: %s)\n\n", string(levels.LevelInfo))

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	// Initialize logger (hqgolog)
	hqgolog.DefaultLogger.SetMaxLevel(levels.LevelStr(verbosity))
	hqgolog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	// Create | Update configuration
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

	// Get sources' keys from configuration
	keys := config.GetKeys()

	// List suported sources
	if listSources {
		hqgolog.Info().Msgf("listing %v current supported sources", au.Underline(strconv.Itoa(len(config.Sources))).Bold())
		hqgolog.Info().Msgf("sources with %v needs a key or token", au.Underline("*").Bold())
		hqgolog.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&keys).Elem()

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

	// Find URLs
	if verbosity != string(levels.LevelSilent) {
		hqgolog.Info().Msgf("finding URLs for %v.", au.Underline(domain).Bold())

		if includeSubdomains {
			hqgolog.Info().Msg("`--include-subdomains` used: includes subdomains' URLs.")
		}

		hqgolog.Print().Msg("")
	}

	options := &xurlfind3r.Options{
		Domain:             domain,
		IncludeSubdomains:  includeSubdomains,
		Sources:            sourcesToUse,
		Keys:               keys,
		ParseWaybackRobots: !skipWaybackRobots,
		ParseWaybackSource: !skipWaybackSource,
		FilterPattern:      filterPattern,
		Matchattern:        matchPattern,
	}

	finder, err := xurlfind3r.New(options)
	if err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	URLs := finder.Find()

	if output != "" {
		// Create output file path directory
		directory := filepath.Dir(output)

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}
		}

		// Create output file
		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		defer file.Close()

		// Write URLs output file and print on screen
		writer := bufio.NewWriter(file)

		for URL := range URLs {
			if verbosity == string(levels.LevelSilent) {
				hqgolog.Print().Msg(URL.Value)
			} else {
				hqgolog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			}

			fmt.Fprintln(writer, URL.Value)
		}

		if err = writer.Flush(); err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}
	} else {
		// Print URLs on screen
		for URL := range URLs {
			if verbosity == string(levels.LevelSilent) {
				hqgolog.Print().Msg(URL.Value)
			} else {
				hqgolog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			}
		}
	}
}
