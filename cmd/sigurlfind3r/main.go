package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
	"github.com/signedsecurity/sigurlfind3r/pkg/runner"
)

type options struct {
	sourcesList bool
	noColor     bool
	silent      bool
}

var (
	co options
	au aurora.Aurora
	so runner.Options
)

func banner() {
	fmt.Fprintln(os.Stderr, aurora.BrightBlue(`
     _                  _  __ _           _ _____
 ___(_) __ _ _   _ _ __| |/ _(_)_ __   __| |___ / _ __
/ __| |/ _`+"`"+` | | | | '__| | |_| | '_ \ / _`+"`"+` | |_ \| '__|
\__ \ | (_| | |_| | |  | |  _| | | | | (_| |___) | |
|___/_|\__, |\__,_|_|  |_|_| |_|_| |_|\__,_|____/|_| v1.0.0
       |___/
`).Bold())
}

func init() {
	flag.StringVar(&so.Domain, "d", "", "")
	flag.StringVar(&so.Domain, "domain", "", "")
	flag.StringVar(&so.SourcesExclude, "es", "", "")
	flag.StringVar(&so.SourcesExclude, "exclude-sources", "", "")
	flag.BoolVar(&so.IncludeSubs, "is", false, "")
	flag.BoolVar(&so.IncludeSubs, "include-subs", false, "")
	flag.BoolVar(&co.sourcesList, "ls", false, "")
	flag.BoolVar(&co.sourcesList, "list-sources", false, "")
	flag.BoolVar(&co.noColor, "ns", false, "")
	flag.BoolVar(&co.noColor, "no-color", false, "")
	flag.BoolVar(&co.silent, "s", false, "")
	flag.BoolVar(&co.silent, "silent", false, "")
	flag.StringVar(&so.SourcesUse, "us", "", "")
	flag.StringVar(&so.SourcesUse, "use-sources", "", "")

	flag.Usage = func() {
		banner()

		h := "USAGE:\n"
		h += "  sigurlfind3r [OPTIONS]\n"

		h += "\nOPTIONS:\n"
		h += "  -d,  --domain            domain to fetch urls for\n"
		h += "  -es, --exclude-sources   comma(,) separated list of sources to exclude\n"
		h += "  -is, --include-subs      include subdomains' urls\n"
		h += "  -ls, --list-sources      list all the available sources\n"
		h += "  -nc, --no-color          no color mode\n"
		h += "  -s,  --silent            silent mode: output urls only\n"
		h += "  -us, --use-sources       comma(,) separated list of sources to use\n\n"

		fmt.Fprintf(os.Stderr, h)
	}

	flag.Parse()

	au = aurora.NewAurora(!co.noColor)
}

func main() {
	options, err := runner.ParseOptions(&so)
	if err != nil {
		log.Fatalln(err)
	}

	if !co.silent {
		banner()
	}

	if co.sourcesList {
		fmt.Println("[", au.BrightBlue("INF"), "] current list of the available", au.Underline(strconv.Itoa(len(options.YAMLConfig.Sources))+" sources").Bold())
		fmt.Println("[", au.BrightBlue("INF"), "] sources marked with an * needs key or token")
		fmt.Println("")

		keys := options.YAMLConfig.GetKeys()
		needsKey := make(map[string]interface{})
		keysElem := reflect.ValueOf(&keys).Elem()

		for i := 0; i < keysElem.NumField(); i++ {
			needsKey[strings.ToLower(keysElem.Type().Field(i).Name)] = keysElem.Field(i).Interface()
		}

		for _, source := range options.YAMLConfig.Sources {
			if _, ok := needsKey[source]; ok {
				fmt.Println(">", source, "*")
			} else {
				fmt.Println(">", source)
			}
		}

		fmt.Println("")
		os.Exit(0)
	}

	if !co.silent {
		fmt.Println("[", au.BrightBlue("INF"), "] fetching urls for", au.Underline(options.Domain).Bold())

		if options.IncludeSubs {
			fmt.Println("[", au.BrightBlue("INF"), "] -iS used: includes subdomains' urls")
		}

		fmt.Println("")
	}

	runner := runner.New(options)

	URLs, err := runner.Run()
	if err != nil {
		log.Fatalln(err)
	}

	for URL := range URLs {
		if co.silent {
			fmt.Println(URL.Value)
		} else {
			fmt.Println(fmt.Sprintf("[%s] %s", au.BrightBlue(URL.Source), URL.Value))
		}
	}
}
