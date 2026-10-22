package bevigil

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getURLsResponse struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.Bevigil

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

		getURLsReqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", domain)

		getURLsRes, err := cfg.Client.NewRequest().
			URL(getURLsReqURL).
			SetHeader("X-Access-Token", key).
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

		for _, URL := range getURLsResData.URLs {
			URL, valid := cfg.Validator(URL)
			if !valid {
				continue
			}

			result := sources.URLResult(s.Name(), URL)

			results <- result
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
