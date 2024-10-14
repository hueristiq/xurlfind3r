package urlscan

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
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

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := cfg.Keys.URLScan.PickRandom()
		if err != nil && !errors.Is(err, sources.ErrNoKeys) {
			result := sources.Result{
				Type:   sources.ResultError,
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

		var after string

		for {
			searchReqURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=100", domain)

			if after != "" {
				searchReqURL += "&search_after=" + after
			}

			searchRes, err := httpclient.Get(searchReqURL, "", searchReqHeaders)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				httpclient.DiscardResponse(searchRes)

				break
			}

			var searchResData searchResponse

			if err = json.NewDecoder(searchRes.Body).Decode(&searchResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				searchRes.Body.Close()

				break
			}

			searchRes.Body.Close()

			if searchResData.Status == 429 {
				break
			}

			for _, result := range searchResData.Results {
				URL := result.Page.URL

				if !cfg.IsInScope(URL) {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}

			if !searchResData.HasMore {
				break
			}

			if len(searchResData.Results) < 1 {
				break
			}

			lastResult := searchResData.Results[len(searchResData.Results)-1]

			if lastResult.Sort != nil {
				var temp []string

				for index := range lastResult.Sort {
					temp = append(temp, cast.ToString(lastResult.Sort[index]))
				}

				after = strings.Join(temp, ",")
			}
		}
	}()

	return results
}

func (source *Source) Name() string {
	return sources.URLSCAN
}
