package main

import (
	"bufio"
	"context"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/hueristiq/hqurlfind3r/internal/configuration"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r"
	"github.com/logrusorgru/aurora/v3"
	flag "github.com/spf13/pflag"
)

var (
	au                 aurora.Aurora
	o                  configuration.CLIOptions
	output             string
	monochrome, silent bool
)

func printBanner() {
	fmt.Fprintln(os.Stderr, configuration.BANNER)
}

func init() {
	flag.StringVarP(&o.Domain, "domain", "d", "", "target domain")
	flag.BoolVar(&o.IncludeSubdomains, "include-subdomains", false, "include subdomains")
	flag.StringVarP(&o.FilterRegex, "filter", "f", "", "URL filtering regex")
	flag.StringSliceVar(&o.SourcesToUse, "use-sources", []string{}, "comma(,) separated sources to use")
	flag.StringSliceVar(&o.SourcesToExclude, "exclude-sources", []string{}, "comma(,) separated sources to exclude")
	flag.BoolVar(&o.ListSources, "list-sources", false, "list all the available sources")
	flag.BoolVarP(&monochrome, "monochrome", "m", false, "no colored output mode")
	flag.BoolVarP(&silent, "silent", "s", false, "silent output mode")
	flag.StringVarP(&output, "output", "o", "", "output file")

	flag.CommandLine.SortFlags = false
	flag.Usage = func() {
		printBanner()

		h := "\nUSAGE:\n"
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

	flag.Parse()

	au = aurora.NewAurora(!monochrome)
}

func main() {
	options, err := configuration.ParseCLIOptions(&o)
	if err != nil {
		log.Fatalln(err)
	}

	if !silent {
		printBanner()
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

	runner := hqurlfind3r.New(&hqurlfind3r.Options{
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
