package zoomeye

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	hqgohttpmethod "github.com/hueristiq/hq-lib-http-go/method"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type searchRequest struct {
	Query    string `json:"qbase64"`
	Page     uint   `json:"page"`
	PageSize uint   `json:"pagesize"`
	SubType  string `json:"sub_type"`
}

type searchResponse struct {
	Total int `json:"total"`
	Data  []struct {
		URL string `json:"url"`
	} `json:"data"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.Zoomeye

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
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

		getURLsReqURL := "https://api.zoomeye.ai/v2/search"

		processed := 0

		for page := uint(1); ; page++ {
			searchReqBody, err := json.Marshal(searchRequest{
				Query:    base64.StdEncoding.EncodeToString([]byte("*." + domain)),
				Page:     page,
				PageSize: 100,
				SubType:  "web",
			})
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to marshal search request: %w", err),
				}

				results <- result

				return
			}

			getURLsRes, err := cfg.Client.NewRequest().
				Method(hqgohttpmethod.POST).
				URL(getURLsReqURL).
				SetHeader("API-KEY", key).
				SetHeader("Content-Type", "application/json").
				Body(searchReqBody).
				Do(ctx)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				return
			}

			var getURLsResData searchResponse

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				// Close errors on a consumed response body are not actionable.
				_ = getURLsRes.Body.Close()

				return
			}

			// Close errors on a consumed response body are not actionable.
			_ = getURLsRes.Body.Close()

			if len(getURLsResData.Data) == 0 {
				break
			}

			for _, record := range getURLsResData.Data {
				var URL string

				var valid bool

				if URL, valid = cfg.Validator(record.URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result

				processed++
			}

			if processed >= getURLsResData.Total {
				break
			}
		}
	}()

	return results
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
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
