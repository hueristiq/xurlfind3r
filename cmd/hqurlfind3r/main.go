package main

import (
	"bufio"
	"regexp"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/hqurlfind3r/internal/configuration"
	"github.com/hueristiq/hqurlfind3r/pkg/runner"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources"
	"github.com/logrusorgru/aurora/v3"
	"github.com/spf13/pflag"
)

var (
	listSources bool

	domain                         string
	sourcesToUse, sourcesToExclude []string
	includeSubdomains              bool
	filterRegex                    string
	monochrome, silent             bool
	output                         string

	au aurora.Aurora
)

func printBanner() {
	fmt.Fprintln(os.Stderr, configuration.BANNER)
}

func init() {
	pflag.StringVarP(&domain, "domain", "d", "", "target domain")
	pflag.BoolVar(&includeSubdomains, "include-subdomains", false, "include subdomains")
	pflag.StringVarP(&filterRegex, "filter", "f", "", "URL filtering regex")
	pflag.StringSliceVar(&sourcesToUse, "use-sources", []string{}, "comma(,) separated sources to use")
	pflag.StringSliceVar(&sourcesToExclude, "exclude-sources", []string{}, "comma(,) separated sources to exclude")
	pflag.BoolVar(&listSources, "list-sources", false, "list all the available sources")
	pflag.BoolVarP(&monochrome, "monochrome", "m", false, "no colored output mode")
	pflag.BoolVarP(&silent, "silent", "s", false, "silent output mode")
	pflag.StringVarP(&output, "output", "o", "", "output file")

	pflag.CommandLine.SortFlags = false
	pflag.Usage = func() {
		printBanner()

		h := "USAGE:\n"
		h += "  hqurlfind3r [OPTIONS]\n"

		h += "\nOPTIONS:\n"
		h += "  -d, --domain string             target domain\n"
		h += "      --include-subdomains        include subdomains\n"
		h += "  -f, --filter string             URL filtering regex\n"
		h += "      --use-sources strings       comma(,) separated sources to use\n"
		h += "      --exclude-sources strings   comma(,) separated sources to exclude\n"
		h += "      --list-sources              list all the available sources\n"
		h += "  -m, --monochrome                no colored output mode\n"
		h += "  -s, --silent                    silent output mode\n"
		h += "  -o, --output string             output file\n"

		fmt.Fprintln(os.Stderr, h)
	}

	pflag.Parse()

	au = aurora.NewAurora(!monochrome)
}

func main() {
	var (
		keys  sources.Keys
		regex *regexp.Regexp
		ftr   filter.Filter
		clr   *collector.Collector
		rnr   *runner.Runner
	)

	if !silent {
		printBanner()
	}

	config, err := configuration.Read()
	if err != nil {
		log.Fatalln(err)
	}

	keys = config.GetKeys()

	if listSources {
		fmt.Println("[", au.BrightBlue("INF"), "] current list of the available", au.Underline(strconv.Itoa(len(config.Sources))+" sources").Bold())
		fmt.Println("[", au.BrightBlue("INF"), "] sources marked with an * needs key or token")
		fmt.Println("")

		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range config.Sources {
			if _, ok := needsKey[source]; ok {
				fmt.Println(">", source, "*")
			} else {
				fmt.Println(">", source)
			}
		}

		fmt.Println("")
		os.Exit(0)
	}

	if !silent {
		fmt.Println("[", au.BrightBlue("INF"), "] fetching urls for", au.Underline(domain).Bold())

		if includeSubdomains {
			fmt.Println("[", au.BrightBlue("INF"), "] `--include-subdomains` used: includes subdomains' urls")
		}

		fmt.Println("")
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
	rnr = runner.New(clr)

	URLs, err := rnr.Run()
	if err != nil {
		log.Fatalln(err)
	}

	if output != "" {
		directory := filepath.Dir(output)

		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err = os.MkdirAll(directory, os.ModePerm); err != nil {
				log.Fatalln(err)
			}
		}

		file, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}

		defer file.Close()

		writer := bufio.NewWriter(file)

		for URL := range URLs {
			if silent {
				fmt.Println(URL.Value)
			} else {
				fmt.Printf("[%s] %s\n", au.BrightBlue(URL.Source), URL.Value)
			}

			fmt.Fprintln(writer, URL.Value)
		}

		if err = writer.Flush(); err != nil {
			log.Fatalln(err)
		}
	} else {
		for URL := range URLs {
			if silent {
				fmt.Println(URL.Value)
			} else {
				fmt.Printf("[%s] %s\n", au.BrightBlue(URL.Source), URL.Value)
			}
		}
	}
}
