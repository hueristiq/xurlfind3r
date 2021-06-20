package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
	"github.com/signedsecurity/sigurlfind3r/internal/configuration"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

var (
	au aurora.Aurora
	o  configuration.CLIOptions
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
	flag.StringVar(&o.Domain, "d", "", "")
	flag.StringVar(&o.Domain, "domain", "", "")
	flag.StringVar(&o.SourcesToExclude, "es", "", "")
	flag.StringVar(&o.SourcesToExclude, "exclude-sources", "", "")
	flag.StringVar(&o.FilterRegex, "f", ".(jpg|jpeg|gif|png|ico|css|eot|tif|tiff|ttf|woff|woff2)", "")
	flag.StringVar(&o.FilterRegex, "filter", ".(jpg|jpeg|gif|png|ico|css|eot|tif|tiff|ttf|woff|woff2)", "")
	flag.BoolVar(&o.IncludeSubdomains, "is", false, "")
	flag.BoolVar(&o.IncludeSubdomains, "include-subs", false, "")
	flag.BoolVar(&o.ListSources, "ls", false, "")
	flag.BoolVar(&o.ListSources, "list-sources", false, "")
	flag.BoolVar(&o.NoColor, "ns", false, "")
	flag.BoolVar(&o.NoColor, "no-color", false, "")
	flag.BoolVar(&o.Silent, "s", false, "")
	flag.BoolVar(&o.Silent, "silent", false, "")
	flag.StringVar(&o.SourcesToUse, "us", "", "")
	flag.StringVar(&o.SourcesToUse, "use-sources", "", "")

	flag.Usage = func() {
		banner()

		h := "USAGE:\n"
		h += "  sigurlfind3r [OPTIONS]\n"

		h += "\nOPTIONS:\n"
		h += "  -d,  --domain            domain to fetch urls for\n"
		h += "  -es, --exclude-sources   comma(,) separated list of sources to exclude\n"
		h += "  -f,  --filter            URL filtering regex\n"
		h += "  -is, --include-subs      include subdomains' urls\n"
		h += "  -ls, --list-sources      list all the available sources\n"
		h += "  -nc, --no-color          no color mode\n"
		h += "  -s,  --silent            silent mode: output urls only\n"
		h += "  -us, --use-sources       comma(,) separated list of sources to use\n"

		fmt.Println(h)
	}

	flag.Parse()

	au = aurora.NewAurora(!o.NoColor)
}

func main() {
	options, err := configuration.ParseCLIOptions(&o)
	if err != nil {
		log.Fatalln(err)
	}

	if !o.Silent {
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

	if !o.Silent {
		fmt.Println("[", au.BrightBlue("INF"), "] fetching urls for", au.Underline(options.Domain).Bold())

		if options.IncludeSubdomains {
			fmt.Println("[", au.BrightBlue("INF"), "] -iS used: includes subdomains' urls")
		}

		fmt.Println("")
	}

	runner := sigurlfind3r.New(&sigurlfind3r.Options{
		FilterRegex:       options.FilterRegex,
		SourcesToUse:      options.SourcesToUse,
		SourcesToExclude:  options.SourcesToExclude,
		IncludeSubdomains: options.IncludeSubdomains,
		Keys: &session.Keys{
			GitHub: options.YAML.Keys.GitHub,
		},
	})

	URLs, err := runner.Run(context.Background(), options.Domain)
	if err != nil {
		log.Fatalln(err)
	}

	for URL := range URLs {
		if o.Silent {
			fmt.Println(URL.Value)
		} else {
			fmt.Printf("[%s] %s\n", au.BrightBlue(URL.Source), URL.Value)
		}
	}
}
