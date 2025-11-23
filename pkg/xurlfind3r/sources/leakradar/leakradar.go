package leakradar

import (
	"encoding/json"
	"fmt"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
)

type getURLsResponse struct {
	Items    []item `json:"items"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type item struct {
	URL         string `json:"url"`
	Occurrences int    `json:"occurrences"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.LEAKRADAR

	return
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		for page := 1; ; page++ {
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

			getURLsReqURL := fmt.Sprintf("https://api.leakradar.io/search/domain/%s/urls", domain)
			getURLsReqCFG := &hqgohttp.RequestConfiguration{
				Headers: []hqgohttp.Header{
					hqgohttp.NewSetHeader("Authorization", "Bearer "+key),
				},
				Params: map[string]string{
					"page":      cast.ToString(page),
					"page_size": "100",
				},
			}

			getURLsRes, err := hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				return
			}

			var getURLsResData getURLsResponse

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				getURLsRes.Body.Close()

				return
			}

			getURLsRes.Body.Close()

			for _, item := range getURLsResData.Items {
				URL := item.URL

				var valid bool

				if URL, valid = cfg.Validator(URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result
			}

			if getURLsResData.PageSize*getURLsResData.Page >= getURLsResData.Total {
				break
			}
		}
	}()

	return results
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
