package urlscan

import (
	"encoding/json"
	"fmt"
	"strings"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgohttpheader "github.com/hueristiq/hq-go-http/header"
	hqgohttpmime "github.com/hueristiq/hq-go-http/mime"
	hqgohttpstatus "github.com/hueristiq/hq-go-http/status"
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

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.URLSCAN

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := s.keys.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to select key: %w", err),
			}

			results <- result

			return
		}

		var after string

		for {
			searchReqURL := "https://urlscan.io/api/v1/search"
			searchReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"q":    "domain:" + domain,
					"size": "10000",
				},
				Headers: []hqgohttp.Header{
					hqgohttp.NewSetHeader(hqgohttpheader.Accept.String(), hqgohttpmime.JSON.String()),
					hqgohttp.NewSetHeader("API-Key", key),
				},
			}

			if after != "" {
				searchReqCFG.Params["search_after"] = after
			}

			searchRes, err := hqgohttp.Get(searchReqURL, searchReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				break
			}

			var searchResData searchResponse

			if err = json.NewDecoder(searchRes.Body).Decode(&searchResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				searchRes.Body.Close()

				break
			}

			searchRes.Body.Close()

			if searchResData.Status == hqgohttpstatus.TooManyRequests.Int() {
				break
			}

			for _, result := range searchResData.Results {
				var URL string

				var valid bool

				if URL, valid = cfg.Validate(result.Page.URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
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

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
