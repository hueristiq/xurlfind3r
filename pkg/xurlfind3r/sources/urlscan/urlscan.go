// Package urlscan implements functions to search URLs from urlscan.
package urlscan

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type response struct {
	Results []struct {
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			err error
			res *fasthttp.Response
		)

		res, err = httpclient.SimpleGet(fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s", config.Domain))
		if err != nil {
			return
		}

		body := res.Body()

		var results response

		if err = json.Unmarshal(body, &results); err != nil {
			return
		}

		for _, i := range results.Results {
			URL := i.Page.URL

			if !sources.IsValid(URL) {
				continue
			}

			if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
				return
			}

			URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "urlscan"
}
