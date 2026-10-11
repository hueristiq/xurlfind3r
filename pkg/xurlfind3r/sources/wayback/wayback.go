package wayback

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgohttpstatus "github.com/hueristiq/hq-lib-http-go/status"
	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	// pageSize is the number of records requested per CDX page.
	pageSize = 10000

	// rateLimitWait is how long to wait before retrying a page after a
	// 429 response.
	rateLimitWait = 60 * time.Second

	// rateLimitMaxRetries caps the number of 429 retries per page.
	rateLimitMaxRetries = 5
)

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Wayback

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		for page := uint(0); ; page++ {
			if err := limiter.Wait(ctx); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to wait on rate limiter: %w", err),
				}

				results <- result

				break
			}

			data, err := getPage(ctx, cfg, domain, page, rateLimitWait)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  err,
				}

				results <- result

				break
			}

			// An empty or header-only page means the results are
			// exhausted. wayback's page counts are not reliable when
			// filters are applied, so the page loop itself detects the
			// end.
			if len(data) <= 1 {
				break
			}

			// Slicing as [1:] to skip the header row.
			for _, record := range data[1:] {
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

// getPage fetches one CDX page for domain, waiting out 429 responses
// for wait each time, up to rateLimitMaxRetries retries.
//
// An empty response body is a clean end-of-results and yields nil data
// and nil error.
//
// Parameters:
//   - ctx (context.Context): the run context.
//   - cfg (*sources.Configuration): the shared source configuration.
//   - domain (string): the target domain.
//   - page (uint): the zero-based CDX page number.
//   - wait (time.Duration): the wait between 429 retries.
//
// Returns:
//   - data ([][]string): the decoded CDX records.
//   - err (error): non-nil on failure.
func getPage(ctx context.Context, cfg *sources.Configuration, domain string, page uint, wait time.Duration) (data [][]string, err error) {
	for attempts := 0; attempts <= rateLimitMaxRetries; attempts++ {
		var getURLsRes *http.Response

		getURLsRes, err = cfg.Client.NewRequest().
			URL("https://web.archive.org/cdx/search/cdx").
			Param("url", "*."+domain+"/*").
			Param("output", "json").
			Param("collapse", "urlkey").
			Param("fl", "timestamp,original,mimetype,statuscode,digest").
			Param("pageSize", cast.ToString(pageSize)).
			Param("page", cast.ToString(page)).
			WithOptions(hqgohttp.WithRequestRetryPolicy(no429RetryPolicy)).
			Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if getURLsRes.StatusCode == hqgohttpstatus.TooManyRequests.Int() {
			// Close errors on a consumed response body are not actionable.
			_ = getURLsRes.Body.Close()

			timer := time.NewTimer(wait)

			select {
			case <-ctx.Done():
				timer.Stop()

				return nil, fmt.Errorf("waiting out rate limit: %w", ctx.Err())
			case <-timer.C:
			}

			continue
		}

		var body []byte

		body, err = io.ReadAll(getURLsRes.Body)

		// Close errors on a consumed response body are not actionable.
		_ = getURLsRes.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if strings.TrimSpace(string(body)) == "" {
			return nil, nil
		}

		if err = json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}

		return data, nil
	}

	return nil, fmt.Errorf("wayback rate limited after %d attempts", rateLimitMaxRetries+1)
}

// no429RetryPolicy disables the client's own 429 retries so wayback can
// wait out the rate limit itself; every other outcome follows the
// client's default retry policy.
//
// Parameters:
//   - ctx (context.Context): the request context.
//   - resp (*http.Response): the attempt's response, or nil.
//   - err (error): the attempt's transport error, or nil.
//
// Returns:
//   - retry (bool): whether the client should retry the attempt.
//   - rerr (error): an error replacing the attempt's outcome; nil to
//     keep it.
func no429RetryPolicy(ctx context.Context, resp *http.Response, err error) (retry bool, rerr error) {
	if ctx.Err() == nil && resp != nil && resp.StatusCode == hqgohttpstatus.TooManyRequests.Int() {
		return false, nil
	}

	return hqgohttp.DefaultRetryPolicy(ctx, resp, err) //nolint:wrapcheck // the policy result is passed through verbatim by design.
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

var limiter = hqgolimiter.New(
	hqgolimiter.WithRate(40, time.Minute),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
