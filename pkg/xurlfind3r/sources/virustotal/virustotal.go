package virustotal

import (
	"context"
	"encoding/json"
	"fmt"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getDomainReportResponse struct {
	DetectedURLs []struct {
		URL string `json:"url"`
	} `json:"detected_urls"`
	Subdomains     []string        `json:"subdomains"`
	UndetectedURLs [][]interface{} `json:"undetected_urls"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.Virustotal

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

		getDomainReportReqURL := "https://www.virustotal.com/vtapi/v2/domain/report"

		if err := limiter.Wait(ctx); err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to wait on rate limiter: %w", err))

			results <- result

			return
		}

		getDomainReportRes, err := cfg.Client.Request().
			URL(getDomainReportReqURL).
			Param("apikey", key).
			Param("domain", domain).
			Context(ctx).Do()
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", sources.RedactError(err, key)))

			results <- result

			return
		}

		if err = sources.CheckResponse(getDomainReportRes.Std(), key); err != nil {
			_ = getDomainReportRes.Body.Close()

			result := sources.ErrorResult(s.Name(), err)

			results <- result

			return
		}

		var getDomainReportResData getDomainReportResponse

		if err = json.NewDecoder(getDomainReportRes.Body).Decode(&getDomainReportResData); err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

			results <- result

			_ = getDomainReportRes.Body.Close()

			return
		}

		_ = getDomainReportRes.Body.Close()

		for _, detectedURL := range getDomainReportResData.DetectedURLs {
			URL, valid := cfg.Validator(detectedURL.URL)
			if !valid {
				continue
			}

			result := sources.URLResult(s.Name(), URL)

			results <- result
		}

		for _, subdomain := range getDomainReportResData.Subdomains {
			URL, valid := cfg.Validator(subdomain)
			if !valid {
				continue
			}

			result := sources.URLResult(s.Name(), URL)

			results <- result
		}

		for _, undetectedURL := range getDomainReportResData.UndetectedURLs {
			if len(undetectedURL) > 0 {
				if URL, ok := undetectedURL[0].(string); ok {
					URL, valid := cfg.Validator(URL)
					if !valid {
						continue
					}

					result := sources.URLResult(s.Name(), URL)

					results <- result
				}
			}
		}
	}()

	return results
}

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(4),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
