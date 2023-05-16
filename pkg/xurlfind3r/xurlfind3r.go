package xurlfind3r

import (
	"fmt"
	"net/url"

	hqurl "github.com/hueristiq/hqgoutils/url"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/sources"
)

type Runner struct {
	Collector *collector.Collector
}

func New(clr *collector.Collector) (runner *Runner) {
	runner = &Runner{
		Collector: clr,
	}

	return
}

func (runner *Runner) Run() (URLs chan sources.URL, err error) {
	URLs = make(chan sources.URL)

	results := runner.Collector.Collect()

	deDupMap := make(map[string]url.Values)
	uniqueMap := make(map[string]sources.URL)

	// Process the results in a separate goroutine
	go func() {
		defer close(URLs)

		for result := range results {
			// unique urls - If the url already exists in the unique map
			if _, exists := uniqueMap[result.Value]; exists {
				continue
			}

			parsedURL, err := hqurl.Parse(result.Value)
			if err != nil {
				continue
			}

			// urls with query
			if len(parsedURL.Query()) > 0 {
				unique := false

				key := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)

				if _, exists := deDupMap[key]; exists {
					for parameter := range parsedURL.Query() {
						if _, exists := deDupMap[key][parameter]; !exists {
							deDupMap[key][parameter] = []string{"xurlfind3r"}
							unique = true
						}
					}
				} else {
					deDupMap[key] = parsedURL.Query()
					unique = true
				}

				if !unique {
					continue
				}
			}

			uniqueMap[parsedURL.String()] = sources.URL{
				Source: result.Source,
				Value:  parsedURL.String(),
			}

			URLs <- uniqueMap[parsedURL.String()]
		}
	}()

	return
}
