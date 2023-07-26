// Package urlscan implements functions to search URLs from urlscan.
package urlscan

import (
	"encoding/json"
	"net/url"
	"strings"

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

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var err error

		var key string

		key, err = sources.PickRandom(config.Keys.URLScan)
		if err != nil {
			return
		}

		reqHeaders := map[string]string{
			"Content-Type": "application/json",
		}

		if key != "" {
			reqHeaders["API-Key"] = key
		}

		var searchAfter []interface{}

		for {
			baseURL := "https://urlscan.io/api/v1/search/"
			params := url.Values{}
			params.Set("q", domain)

			if searchAfter != nil {
				searchAfterJSON, _ := json.Marshal(searchAfter)
				params.Set("search_after", string(searchAfterJSON))
			}

			reqURL := baseURL + "?" + params.Encode()

			var res *fasthttp.Response

			res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", reqHeaders, nil)
			if err != nil {
				return
			}

			var responseData response

			if err = json.Unmarshal(res.Body(), &responseData); err != nil {
				return
			}

			if responseData.Status == 429 {
				break
			}

			for _, result := range responseData.Results {
				URL := result.Page.URL

				if result.Page.Domain != domain ||
					!strings.HasSuffix(result.Page.Domain, domain) {
					continue
				}

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}

			if !responseData.HasMore {
				break
			}

			lastResult := responseData.Results[len(responseData.Results)-1]
			searchAfter = lastResult.Sort
		}
	}()

	return
}

func (source *Source) Name() string {
	return "urlscan"
}
