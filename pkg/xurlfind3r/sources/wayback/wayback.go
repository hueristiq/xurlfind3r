package wayback

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.WAYBACK

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		for page := uint(0); ; page++ {
			getURLsReqURL := "https://web.archive.org/cdx/search/cdx"

			if err := limiter.Wait(ctx); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to wait on rate limiter: %w", err),
				}

				results <- result

				break
			}

			getURLsRes, err := cfg.Client.NewRequest().
				URL(getURLsReqURL).
				Param("url", "*."+domain+"/*").
				Param("output", "json").
				Param("collapse", "urlkey").
				Param("fl", "timestamp,original,mimetype,statuscode,digest").
				Param("pageSize", "100").
				Param("page", cast.ToString(page)).
				Do(ctx)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				break
			}

			var getURLsResData [][]string

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				// Close errors on a consumed response body are not actionable.
				_ = getURLsRes.Body.Close()

				break
			}

			// Close errors on a consumed response body are not actionable.
			_ = getURLsRes.Body.Close()

			// check if there's results, wayback's pagination response
			// is not always correct when using a filter
			if len(getURLsResData) == 0 {
				break
			}

			// Slicing as [1:] to skip first result by default
			for _, record := range getURLsResData[1:] {
				var URL string

				var valid bool

				if URL, valid = cfg.Validator(record[1]); !valid {
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
	}()

	return results
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(40),
	hqgolimiter.WithRate(1, 30*time.Second),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
