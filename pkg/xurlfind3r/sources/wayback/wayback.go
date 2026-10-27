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

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Wayback

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

		previousSignature := ""

		for page := uint(0); ; page++ {
			if page >= sources.MaxPages {
				result := sources.WarningResult(s.Name(), fmt.Sprintf("pagination capped at %d pages", sources.MaxPages))

				results <- result

				break
			}

			if err := limiter.Wait(ctx); err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to wait on rate limiter: %w", err))

				results <- result

				break
			}

			data, err := getPage(ctx, cfg, domain, page, rateLimitWait)
			if err != nil {
				result := sources.ErrorResult(s.Name(), err)

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

			signature := fmt.Sprintf("%d:%v:%v", len(data), data[1], data[len(data)-1])

			if signature == previousSignature {
				result := sources.WarningResult(s.Name(), "pagination stopped: page contents repeated")

				results <- result

				break
			}

			previousSignature = signature

			// Slicing as [1:] to skip the header row.
			for _, record := range data[1:] {
				if len(record) < 2 {
					continue
				}

				URL, valid := cfg.Validator(record[1])
				if !valid {
					continue
				}

				result := sources.URLResult(s.Name(), URL)

				results <- result
			}
		}
	}()

	return results
}

func getPage(ctx context.Context, cfg *sources.Configuration, domain string, page uint, wait time.Duration) (data [][]string, err error) {
	for attempts := 0; attempts <= rateLimitMaxRetries; attempts++ {
		var getURLsRes *hqgohttp.Response

		getURLsRes, err = cfg.Client.Request().
			URL("https://web.archive.org/cdx/search/cdx").
			Param("url", "*."+domain+"/*").
			Param("output", "json").
			Param("collapse", "urlkey").
			Param("fl", "timestamp,original,mimetype,statuscode,digest").
			Param("pageSize", cast.ToString(pageSize)).
			Param("page", cast.ToString(page)).
			WithOptions(hqgohttp.WithRequestRetryPolicy(no429RetryPolicy)).
			Context(ctx).Do()
		if err != nil {
			if getURLsRes != nil {
				_ = getURLsRes.Body.Close()
			}

			return nil, fmt.Errorf("request failed: %w", err)
		}

		if getURLsRes.StatusCode == hqgohttpstatus.TooManyRequests.Int() {
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

		if err = sources.CheckResponse(getURLsRes.Std()); err != nil { //nolint:bodyclose // the body belongs to getURLsRes and is closed below.
			_ = getURLsRes.Body.Close()

			return nil, fmt.Errorf("request failed: %w", err)
		}

		var body []byte

		body, err = io.ReadAll(getURLsRes.Body)

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

func no429RetryPolicy(ctx context.Context, req *http.Request, resp *http.Response, err error, attempt int) (retry bool, rerr error) {
	if ctx.Err() == nil && resp != nil && resp.StatusCode == hqgohttpstatus.TooManyRequests.Int() {
		return false, nil
	}

	return hqgohttp.DefaultRetryPolicy(ctx, req, resp, err, attempt) //nolint:wrapcheck // the policy result is passed through verbatim by design.
}

var limiter = hqgolimiter.New(
	hqgolimiter.WithRate(40, time.Minute),
)

var _ sources.Source = (*Source)(nil)

const (
	pageSize            = 10000
	rateLimitWait       = 60 * time.Second
	rateLimitMaxRetries = 5
)

func New() (source sources.Source) {
	source = &Source{}

	return
}
