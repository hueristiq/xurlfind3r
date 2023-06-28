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

type Response struct {
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
			err error
			key string

			searchAfter []interface{}

			// res     *fasthttp.Response
			resData Response
		)

		key, err = sources.PickRandom(config.Keys.URLScan)
		if key == "" || err != nil {
			return
		}

		reqHeaders := map[string]string{
			"Content-Type": "application/json",
		}

		if len(config.Keys.URLScan) > 0 {
			reqHeaders["API-Key"] = key
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

			var res *fasthttp.Response

			res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", reqHeaders, nil)
			if err != nil {
				return
			}

			var data Response

			if err = json.Unmarshal(res.Body(), &data); err != nil {
				return
			}

			if data.Status == 429 {
				break
			}

			for index := range data.Results {
				URL := data.Results[index].Page.URL

				if data.Results[index].Page.Domain == config.Domain ||
					strings.HasSuffix(data.Results[index].Page.Domain, config.Domain) {
					URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
				}

				// if !sources.IsValid(URL) {
				// 	continue
				// }

				// if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
				// 	return
				// }

				// URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}

			if !resData.HasMore {
				break
			}

			lastResult := resData.Results[len(resData.Results)-1]
			searchAfter = lastResult.Sort
		}
	}()

	return
}

func (source *Source) Name() string {
	return "urlscan"
}
