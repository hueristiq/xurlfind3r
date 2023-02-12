package urlscan

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/output"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/requests"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/sources"
)

type response struct {
	Results []struct {
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

type Source struct{}

func (source *Source) Run(keys sources.Keys, ftr filter.Filter) chan output.URL {
	domain := ftr.Domain

	URLs := make(chan output.URL)

	go func() {
		defer close(URLs)

		res, err := requests.SimpleGet(fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s", domain))
		if err != nil {
			return
		}

		body := res.Body()

		var results response

		if err := json.Unmarshal(body, &results); err != nil {
			return
		}

		for _, i := range results.Results {
			if URL, ok := ftr.Examine(i.Page.URL); ok {
				URLs <- output.URL{Source: source.Name(), Value: URL}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "urlscan"
}
