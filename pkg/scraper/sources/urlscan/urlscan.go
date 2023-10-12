package urlscan

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
	"github.com/valyala/fasthttp"
)

type searchResponse struct {
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

func (source *Source) Run(config *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		var err error

		var key string

		key, err = sources.PickRandom(config.Keys.URLScan)
		if err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		searchReqHeaders := map[string]string{
			"Content-Type": "application/json",
		}

		if key != "" {
			searchReqHeaders["API-Key"] = key
		}

		var searchAfter []interface{}

		for {
			after := ""

			if searchAfter != nil {
				var searchAfterJSON []byte

				searchAfterJSON, err = json.Marshal(searchAfter)
				if err != nil {
					result := sources.Result{
						Type:   sources.Error,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					return
				}

				after = "&search_after=" + string(searchAfterJSON)
			}

			searchReqURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=100", domain) + after

			var searchRes *fasthttp.Response

			searchRes, err = httpclient.Get(searchReqURL, "", searchReqHeaders)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			var searchResData searchResponse

			err = json.Unmarshal(searchRes.Body(), &searchResData)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			if searchResData.Status == 429 {
				break
			}

			for _, result := range searchResData.Results {
				URL := result.Page.URL

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					continue
				}

				result := sources.Result{
					Type:   sources.URL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}

			if !searchResData.HasMore {
				break
			}

			lastResult := searchResData.Results[len(searchResData.Results)-1]
			searchAfter = lastResult.Sort
		}
	}()

	return results
}

func (source *Source) Name() string {
	return "urlscan"
}
