package leakradar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
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
	name = sources.Leakradar

	return
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := s.keys.PickRandom()
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to select key: %w", err))

			results <- result

			return
		}

		previousSignature := ""

		for page := 1; ; page++ {
			if page > sources.MaxPages {
				result := sources.WarningResult(s.Name(), fmt.Sprintf("pagination capped at %d pages", sources.MaxPages))

				results <- result

				break
			}

			getURLsReqURL := fmt.Sprintf("https://api.leakradar.io/search/domain/%s/urls", domain)

			getURLsRes, err := cfg.Client.NewRequest().
				URL(getURLsReqURL).
				Param("page", cast.ToString(page)).
				Param("page_size", "100").
				SetHeader("Authorization", "Bearer "+key).
				Do(ctx)
			if err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

				results <- result

				return
			}

			if err = sources.CheckResponse(getURLsRes, key); err != nil {
				_ = getURLsRes.Body.Close()

				result := sources.ErrorResult(s.Name(), err)

				results <- result

				return
			}

			var getURLsResData getURLsResponse

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

				results <- result

				_ = getURLsRes.Body.Close()

				return
			}

			_ = getURLsRes.Body.Close()

			signature := fmt.Sprintf("%d|%d", len(getURLsResData.Items), getURLsResData.Total)

			if len(getURLsResData.Items) > 0 {
				signature += "|" + getURLsResData.Items[0].URL + "|" + getURLsResData.Items[len(getURLsResData.Items)-1].URL
			}

			if signature == previousSignature {
				result := sources.WarningResult(s.Name(), "pagination stopped: page watermark did not advance")

				results <- result

				break
			}

			previousSignature = signature

			for _, item := range getURLsResData.Items {
				URL, valid := cfg.Validator(item.URL)
				if !valid {
					continue
				}

				result := sources.URLResult(s.Name(), URL)

				results <- result
			}

			if len(getURLsResData.Items) < 1 || getURLsResData.PageSize*getURLsResData.Page >= getURLsResData.Total {
				break
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
