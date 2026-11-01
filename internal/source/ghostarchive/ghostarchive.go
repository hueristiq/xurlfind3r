// Package ghostarchive implements the Ghost Archive search provider.
package ghostarchive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

var resultPattern = regexp.MustCompile(`<a href="/archive/[^"]*">([^<]+)</a>`)

// Client is an immutable Ghost Archive client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	wait func(context.Context) error
}

// Option customizes a Client during construction.
type Option func(*Client) error

// WithRateLimitWait replaces the default 30-requests-per-minute wait policy.
func WithRateLimitWait(wait func(context.Context) error) Option {
	return func(client *Client) error {
		if wait == nil {
			return errors.New("ghostarchive: nil rate-limit wait function")
		}

		client.wait = wait

		return nil
	}
}

// New constructs a client with its own rate limiter.
func New(doer hqgosource.Doer, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.New("ghostarchive: nil HTTP client")
	}

	limiter := hqgolimiter.New(hqgolimiter.WithRate(30, time.Minute))
	client := &Client{doer: doer, wait: limiter.Wait}

	for _, apply := range options {
		if apply == nil {
			return nil, errors.New("ghostarchive: nil option")
		}

		if err := apply(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// Search streams archived URLs. Repeated pages indicate exhaustion; at most
// 100 pages are requested.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("ghostarchive: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("ghostarchive: empty domain")})

			return
		}

		term := domain
		if strings.Count(domain, ".") == 1 {
			term = "." + domain
		}

		previous := ""
		finished := false

		for page := range maxPages {
			if err := c.wait(ctx); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("ghostarchive: rate-limit wait: %w", err)})

				return
			}

			query := url.Values{"term": {term}, "page": {strconv.Itoa(page)}}

			response, err := request.Do(ctx, c.doer, http.MethodGet, "https://ghostarchive.org/search", http.NoBody, query, nil) //nolint:bodyclose // request.Read closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			body, err := request.Read(response)
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			source := string(body)
			if source == previous {
				finished = true

				break
			}

			previous = source

			if strings.Contains(source, "The site is under maintenance") || strings.Contains(source, "No archives for that site") {
				stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("ghostarchive: no archives for that site")})

				return
			}

			matches := resultPattern.FindAllStringSubmatch(source, -1)
			if len(matches) == 0 {
				finished = true

				break
			}

			for _, match := range matches {
				if !stream.Emit(ctx, results, hqgosource.Result{Value: match[1]}) {
					return
				}
			}

			if !strings.Contains(source, "Next Page") && !strings.Contains(source, ">»</a>") {
				finished = true

				break
			}
		}

		if !finished {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}
