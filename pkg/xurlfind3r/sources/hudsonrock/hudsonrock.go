package hudsonrock

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getURLsResponse struct {
	Data struct {
		EmployeesUrls []struct {
			URL string `json:"url"`
		} `json:"employees_urls"`
		ClientsUrls []struct {
			URL string `json:"url"`
		} `json:"clients_urls"`
	} `json:"data"`
}

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Hudsonrock

	return
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		getURLsReqURL := "https://cavalier.hudsonrock.com/api/json/v2/osint-tools/urls-by-domain"

		getURLsRes, err := cfg.Client.NewRequest().
			URL(getURLsReqURL).
			Param("domain", domain).
			Do(ctx)
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

			results <- result

			return
		}

		if err = sources.CheckResponse(getURLsRes); err != nil {
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

		for _, record := range append(getURLsResData.Data.EmployeesUrls, getURLsResData.Data.ClientsUrls...) {
			URL, valid := cfg.Validator(record.URL)
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
	source = &Source{}

	return
}
