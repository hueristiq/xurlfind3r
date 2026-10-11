package virustotal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
	name = sources.VIRUSTOTAL

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

		getDomainReportReqURL := "https://www.virustotal.com/vtapi/v2/domain/report"

		if err := limiter.Wait(ctx); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to wait on rate limiter: %w", err),
			}

			results <- result

			return
		}

		getDomainReportRes, err := cfg.Client.NewRequest().
			URL(getDomainReportReqURL).
			Param("apikey", key).
			Param("domain", domain).
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

		var getDomainReportResData getDomainReportResponse

		if err = json.NewDecoder(getDomainReportRes.Body).Decode(&getDomainReportResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to parse JSON response: %w", err),
			}

			results <- result

			// Close errors on a consumed response body are not actionable.
			_ = getDomainReportRes.Body.Close()

			return
		}

		// Close errors on a consumed response body are not actionable.
		_ = getDomainReportRes.Body.Close()

		for _, detectedURL := range getDomainReportResData.DetectedURLs {
			var URL string

			var valid bool

			if URL, valid = cfg.Validator(detectedURL.URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: s.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, subdomain := range getDomainReportResData.Subdomains {
			var URL string

			var valid bool

			if URL, valid = cfg.Validator(subdomain); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: s.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, undetectedURL := range getDomainReportResData.UndetectedURLs {
			if len(undetectedURL) > 0 {
				if URL, ok := undetectedURL[0].(string); ok {
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

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(4),
	hqgolimiter.WithRate(1, 30*time.Second),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
