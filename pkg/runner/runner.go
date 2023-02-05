package runner

import (
	"fmt"
	urlz "net/url"

	"github.com/hueristiq/hqgoutils/url"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/output"
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

func (runner *Runner) Run() (URLs chan output.URL, err error) {
	URLs = make(chan output.URL)

	results := runner.Collector.Collect()

	deDupMap := make(map[string]urlz.Values)
	uniqueMap := make(map[string]output.URL)

	// Process the results in a separate goroutine
	go func() {
		defer close(URLs)

		for result := range results {
			// unique urls - If the url already exists in the unique map
			if _, exists := uniqueMap[result.Value]; exists {
				continue
			}

			parsedURL, err := url.Parse(url.Options{URL: result.Value})
			if err != nil {
				continue
			}

			// urls with query
			if len(parsedURL.Query()) > 0 {
				unique := false

				key := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, parsedURL.Path)

				fmt.Println(key)

				if _, exists := deDupMap[key]; exists {
					for parameter := range parsedURL.Query() {
						if _, exists := deDupMap[key][parameter]; !exists {
							deDupMap[key][parameter] = []string{"hqurlfind3r"}
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

			uniqueMap[parsedURL.String()] = output.URL{
				Source: result.Source,
				Value:  parsedURL.String(),
			}

			URLs <- uniqueMap[parsedURL.String()]
		}
	}()

	return
}
