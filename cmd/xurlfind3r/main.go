package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"dario.cat/mergo"
	hqgolog "github.com/hueristiq/hqgolog"
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

	domain            string
	includeSubdomains bool
	listSources       bool
	sourcesToUse      []string
	skipWaybackRobots bool
	skipWaybackSource bool
	monochrome        bool
	output            string
	verbosity         string
)

func init() {
	// Handle command line arguments & flags
	pflag.StringVarP(&domain, "domain", "d", "", "")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "")
	pflag.BoolVar(&listSources, "list-sources", false, "")
	pflag.StringSliceVarP(&sourcesToUse, "sources", "s", sources.List, "")
	pflag.BoolVar(&skipWaybackRobots, "skip-wayback-robots", false, "")
	pflag.BoolVar(&skipWaybackSource, "skip-wayback-source", false, "")
	pflag.BoolVarP(&monochrome, "monochrome", "m", false, "")
	pflag.StringVarP(&output, "output", "o", "", "")
	pflag.StringVarP(&verbosity, "verbosity", "v", string(levels.LevelInfo), "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "USAGE:\n"
		h += "  xurlfind3r [OPTIONS]\n"

		h += "\nTARGET:\n"
		h += "  -d, --domain string             target domain\n"
		h += "      --include-subdomains bool   include domain's subdomains\n"

		h += "\nSOURCES:\n"
		h += "      --list-sources bool         list available sources\n"
		h += " -s   --sources strings           comma(,) separated sources to use (default: commoncrawl,github,intelx,otx,urlscan,wayback)\n"

		h += "\nCONFIGURATION:\n"
		h += "      --skip-wayback-robots bool  skip parsing wayback robots.txt snapshots\n"
		h += "      --skip-wayback-source bool  skip parsing wayback source code snapshots\n"

		h += "\nOUTPUT:\n"
		h += "  -m, --monochrome                no colored output mode\n"
		h += "  -o, --output string             output file to write found URLs\n"
		h += fmt.Sprintf("  -v, --verbosity                 debug, info, warning, error, fatal or silent (default: %s)\n\n", string(levels.LevelInfo))

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	// Initialize logger
	hqgolog.DefaultLogger.SetMaxLevel(levels.LevelStr(verbosity))
	hqgolog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	// Handle configuration on initial run
	var (
		err    error
		config configuration.Configuration
	)

	_, err = os.Stat(configuration.ConfigurationFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			config = configuration.Default

			if err = configuration.Write(&config); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}
		} else {
			hqgolog.Fatal().Msg(err.Error())
		}
	} else {
		config, err = configuration.Read()
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		if config.Version != configuration.VERSION {
			if err = mergo.Merge(&config, configuration.Default); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}

			config.Version = configuration.VERSION

			if err = configuration.Write(&config); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}
		}
	}

	au = aurora.NewAurora(!monochrome)
}

func main() {
	if verbosity != string(levels.LevelSilent) {
		fmt.Fprintln(os.Stderr, configuration.BANNER)
	}

	config, err := configuration.Read()
	if err != nil {
		hqgolog.Fatal().Msg(err.Error())
	}

	keys := config.GetKeys()

	// Handle sources listing
	if listSources {
		hqgolog.Info().Msgf("current list of the available %v sources", au.Underline(strconv.Itoa(len(config.Sources))).Bold())
		hqgolog.Info().Msg("sources marked with an * needs key or token")
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

	// Handle URLs finding
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
	}

	finder := xurlfind3r.New(options)
	URLs := finder.Find()

	if output != "" {
		directory := filepath.Dir(output)

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				hqgolog.Fatal().Msg(err.Error())
			}
		}

		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hqgolog.Fatal().Msg(err.Error())
		}

		defer file.Close()

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
		for URL := range URLs {
			if verbosity == string(levels.LevelSilent) {
				hqgolog.Print().Msg(URL.Value)
			} else {
				hqgolog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			}
		}
	}
}
