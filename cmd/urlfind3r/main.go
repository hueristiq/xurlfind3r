package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/urlfind3r/internal/configuration"
	"github.com/hueristiq/urlfind3r/pkg/urlfind3r"
	"github.com/logrusorgru/aurora/v3"
)

var (
	au      aurora.Aurora
	o       configuration.CLIOptions
	output  string
	silent  bool
	noColor bool
)

func banner() {
	fmt.Fprintln(os.Stderr, configuration.BANNER)
}

func init() {
	flag.StringVar(&o.Domain, "d", "", "")
	flag.StringVar(&o.Domain, "domain", "", "")
	flag.StringVar(&o.SourcesToExclude, "eS", "", "")
	flag.StringVar(&o.SourcesToExclude, "exclude-sources", "", "")
	flag.StringVar(&o.FilterRegex, "f", "", "")
	flag.StringVar(&o.FilterRegex, "filter", "", "")
	flag.BoolVar(&o.IncludeSubdomains, "iS", false, "")
	flag.BoolVar(&o.IncludeSubdomains, "include-subs", false, "")
	flag.BoolVar(&o.ListSources, "lS", false, "")
	flag.BoolVar(&o.ListSources, "list-sources", false, "")
	flag.BoolVar(&noColor, "nC", false, "")
	flag.BoolVar(&noColor, "no-color", false, "")
	flag.BoolVar(&silent, "s", false, "")
	flag.BoolVar(&silent, "silent", false, "")
	flag.StringVar(&o.SourcesToUse, "uS", "", "")
	flag.StringVar(&o.SourcesToUse, "use-sources", "", "")
	flag.StringVar(&output, "o", "", "")
	flag.StringVar(&output, "output", "", "")

	flag.Usage = func() {
		banner()

		h := "USAGE:\n"
		h += "  urlfind3r [OPTIONS]\n"

		h += "\nOPTIONS:\n"
		h += "   -d, --domain            domain to fetch urls for\n"
		h += "  -eS, --exclude-sources   comma(,) separated list of sources to exclude\n"
		h += "   -f, --filter            URL filtering regex\n"
		h += "  -iS, --include-subs      include subdomains' urls\n"
		h += "  -lS, --list-sources      list all the available sources\n"
		h += "  -nC, --no-color          no color mode\n"
		h += "   -s  --silent            silent mode: output urls only\n"
		h += "  -uS, --use-sources       comma(,) separated list of sources to use\n"
		h += "   -o, --output            output file\n"

		fmt.Println(h)
	}

	flag.Parse()

	au = aurora.NewAurora(!noColor)
}

func main() {
	options, err := configuration.ParseCLIOptions(&o)
	if err != nil {
		log.Fatalln(err)
	}

	if !silent {
		banner()
	}

	if o.ListSources {
		fmt.Println("[", au.BrightBlue("INF"), "] current list of the available", au.Underline(strconv.Itoa(len(options.YAML.Sources))+" sources").Bold())
		fmt.Println("[", au.BrightBlue("INF"), "] sources marked with an * needs key or token")
		fmt.Println("")

		keys := options.YAML.GetKeys()
		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range options.YAML.Sources {
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
		fmt.Println("[", au.BrightBlue("INF"), "] fetching urls for", au.Underline(options.Domain).Bold())

		if options.IncludeSubdomains {
			fmt.Println("[", au.BrightBlue("INF"), "] -iS used: includes subdomains' urls")
		}

		fmt.Println("")
	}

	runner := urlfind3r.New(&urlfind3r.Options{
		FilterRegex:       options.FilterRegex,
		SourcesToUse:      options.SourcesToUse,
		SourcesToExclude:  options.SourcesToExclude,
		IncludeSubdomains: options.IncludeSubdomains,
		Keys:              options.YAML.GetKeys(),
	})

	URLs, err := runner.Run(context.Background(), options.Domain)
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
