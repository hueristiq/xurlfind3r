package main

import (
	"bufio"
	"regexp"

	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	hqlog "github.com/hueristiq/hqgoutils/log"
	"github.com/hueristiq/hqgoutils/log/formatter"
	"github.com/hueristiq/hqgoutils/log/levels"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/filter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/sources"
	"github.com/imdario/mergo"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
)

var (
	au aurora.Aurora

	listSources bool

	domain                         string
	sourcesToUse, sourcesToExclude []string
	includeSubdomains              bool
	filterRegex                    string
	output                         string

	monochrome bool
	verbosity  string
)

func init() {
	// parse flags
	pflag.StringVarP(&domain, "domain", "d", "", "target domain")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "include subdomains")
	pflag.StringVarP(&filterRegex, "filter", "f", "", "URL filtering regex")
	pflag.StringSliceVar(&sourcesToUse, "use-sources", []string{}, "comma(,) separated sources to use")
	pflag.StringSliceVar(&sourcesToExclude, "exclude-sources", []string{}, "comma(,) separated sources to exclude")
	pflag.BoolVar(&listSources, "list-sources", false, "list all the available sources")
	pflag.BoolVarP(&monochrome, "monochrome", "m", false, "no colored output mode")
	pflag.StringVarP(&output, "output", "o", "", "output file")
	pflag.StringVarP(&verbosity, "verbosity", "v", string(levels.LevelInfo), "")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		fmt.Fprintln(os.Stderr, configuration.BANNER)

		h := "USAGE:\n"
		h += "  xurlfind3r [OPTIONS]\n"

		h += "\nINPUT:\n"
		h += "  -d, --domain string             target domain\n"

		h += "\nSOURCES:\n"
		h += "      --use-sources strings       comma(,) separated sources to use\n"
		h += "      --exclude-sources strings   comma(,) separated sources to exclude\n"
		h += "      --list-sources              list all the available sources\n"

		h += "\nFILTER:\n"

		h += "      --include-subdomains        include subdomains\n"
		h += "  -f, --filter string             URL filtering regex\n"

		h += "\nOUTPUT:\n"
		h += "  -m, --monochrome                no colored output mode\n"
		h += "  -o, --output string             output file to write found URLs\n"
		h += fmt.Sprintf("  -v, --verbosity                 debug, info, warning, error, fatal or silent (default: %s)\n\n", string(levels.LevelInfo))

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	// initialize logger
	hqlog.DefaultLogger.SetMaxLevel(levels.LevelStr(verbosity))
	hqlog.DefaultLogger.SetFormatter(formatter.NewCLI(&formatter.CLIOptions{
		Colorize: !monochrome,
	}))

	// initialize configuration
	var (
		err  error
		conf configuration.Configuration
	)

	_, err = os.Stat(configuration.ConfigurationFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			conf = configuration.Default

			if err = configuration.Write(&conf); err != nil {
				hqlog.Fatal().Msg(err.Error())
			}
		} else {
			hqlog.Fatal().Msg(err.Error())
		}
	} else {
		conf, err = configuration.Read()
		if err != nil {
			hqlog.Fatal().Msg(err.Error())
		}

		if conf.Version != configuration.VERSION {
			if err = mergo.Merge(&conf, configuration.Default); err != nil {
				hqlog.Fatal().Msg(err.Error())
			}

			conf.Version = configuration.VERSION

			if err = configuration.Write(&conf); err != nil {
				hqlog.Fatal().Msg(err.Error())
			}
		}
	}

	au = aurora.NewAurora(!monochrome)
}

func main() {
	var (
		keys  sources.Keys
		regex *regexp.Regexp
		ftr   filter.Filter
		clr   *collector.Collector
		rnr   *xurlfind3r.Runner
	)

	if verbosity != string(levels.LevelSilent) {
		fmt.Fprintln(os.Stderr, configuration.BANNER)
	}

	config, err := configuration.Read()
	if err != nil {
		hqlog.Fatal().Msg(err.Error())
	}

	keys = config.GetKeys()

	if listSources {
		hqlog.Info().Msgf("current list of the available %v sources", au.Underline(strconv.Itoa(len(config.Sources))).Bold())
		hqlog.Info().Msg("sources marked with an * needs key or token")
		hqlog.Print().Msg("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range config.Sources {
			if _, ok := needsKey[source]; ok {
				hqlog.Print().Msgf("> %s *", source)
			} else {
				hqlog.Print().Msgf("> %s", source)
			}
		}

		hqlog.Print().Msg("")
		os.Exit(0)
	}

	if verbosity != string(levels.LevelSilent) {
		hqlog.Info().Msgf("`fetching urls for %v", au.Underline(domain).Bold())

		if includeSubdomains {
			hqlog.Info().Msg("`--include-subdomains` used: includes subdomains' urls")
		}

		hqlog.Print().Msg("")
	}

	if filterRegex != "" {
		regex = regexp.MustCompile(filterRegex)
	}

	ftr = filter.Filter{
		Domain:            domain,
		IncludeSubdomains: includeSubdomains,
		ExcludeRegex:      regex,
	}

	clr = collector.New(sourcesToUse, sourcesToExclude, keys, ftr)
	rnr = xurlfind3r.New(clr)

	URLs, err := rnr.Run()
	if err != nil {
		hqlog.Fatal().Msg(err.Error())
	}

	if output != "" {
		directory := filepath.Dir(output)

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				hqlog.Fatal().Msg(err.Error())
			}
		}

		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			hqlog.Fatal().Msg(err.Error())
		}

		defer file.Close()

		writer := bufio.NewWriter(file)

		for URL := range URLs {
			if verbosity == string(levels.LevelSilent) {
				hqlog.Print().Msg(URL.Value)
			} else {
				hqlog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			}

			fmt.Fprintln(writer, URL.Value)
		}

		if err = writer.Flush(); err != nil {
			hqlog.Fatal().Msg(err.Error())
		}
	} else {
		for URL := range URLs {
			if verbosity == string(levels.LevelSilent) {
				hqlog.Print().Msg(URL.Value)
			} else {
				hqlog.Print().Msgf("[%s] %s", au.BrightBlue(URL.Source), URL.Value)
			}
		}
	}
}
