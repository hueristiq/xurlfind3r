// Package urlscan implements functions to search URLs from urlscan.
package urlscan

import (
	"encoding/json"
	"net/url"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type response struct {
	Results []struct {
		Page struct {
			Domain   string `json:"domain"`
			MimeType string `json:"mimeType"`
			URL      string `json:"url"`
			Status   string `json:"status"`
		} `json:"page"`
		Sort []interface{} `json:"sort"`
	} `json:"results"`
	Status  int  `json:"status"`
	Total   int  `json:"total"`
	Took    int  `json:"took"`
	HasMore bool `json:"has_more"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			key         string
			err         error
			res         *fasthttp.Response
			searchAfter []interface{}
			headers     = map[string]string{
				"Content-Type": "application/json",
			}
		)

		key, err = sources.PickRandom(config.Keys.URLScan)
		if key == "" || err != nil {
			return
		}

		if len(config.Keys.URLScan) > 0 {
			headers["API-Key"] = key
		}

		for {
			baseURL := "https://urlscan.io/api/v1/search/"
			params := url.Values{}
			params.Set("q", config.Domain)

			if searchAfter != nil {
				searchAfterJSON, _ := json.Marshal(searchAfter)
				params.Set("search_after", string(searchAfterJSON))
			}

			reqURL := baseURL + "?" + params.Encode()

			res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", headers, nil)
			if err != nil {
				return
			}

			body := res.Body()

			var results response

			if err = json.Unmarshal(body, &results); err != nil {
				return
			}

			if results.Status == 429 {
				break
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

			if !results.HasMore {
				break
			}

			lastResult := results.Results[len(results.Results)-1]
			searchAfter = lastResult.Sort
		}
	}()

	return
}

func (source *Source) Name() string {
	return "urlscan"
}
