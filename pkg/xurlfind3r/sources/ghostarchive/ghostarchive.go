package ghostarchive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	hqgohttpstatus "github.com/hueristiq/hq-lib-http-go/status"
	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Ghostarchive

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

		term := domain

		if strings.Count(domain, ".") == 1 {
			term = "." + domain
		}

		var previousSrc string

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

			getURLsReqURL := "https://ghostarchive.org/search"

			getURLsRes, err := cfg.Client.NewRequest().
				URL(getURLsReqURL).
				Param("term", term).
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

			status := getURLsRes.StatusCode

			body, err := io.ReadAll(getURLsRes.Body)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to read response body: %w", err),
				}

				results <- result

				_ = getURLsRes.Body.Close()

				break
			}

			_ = getURLsRes.Body.Close()

			if status == hqgohttpstatus.TooManyRequests.Int() || status == hqgohttpstatus.ServiceUnavailable.Int() {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed with status: %v", status),
				}

				results <- result

				break
			}

			src := string(body)

			// The search API repeats the last available page for any
			// out-of-range page number, so an unchanged page means the
			// results are exhausted.
			if src == previousSrc {
				break
			}

			previousSrc = src

			if strings.Contains(src, "The site is under maintenance") || strings.Contains(src, "No archives for that site") {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  errors.New("no archives for that site"),
				}

				results <- result

				break
			}

			matches := resultPattern.FindAllStringSubmatch(src, -1)

			if len(matches) == 0 {
				break
			}

			for _, match := range matches {
				var URL string

				var valid bool

				if URL, valid = cfg.Validator(match[1]); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result
			}

			if !strings.Contains(src, "Next Page") && !strings.Contains(src, ">»</a>") {
				break
			}
		}
	}()

	return results
}

var resultPattern = regexp.MustCompile(`<a href="/archive/[^"]*">([^<]+)</a>`)

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(30),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
