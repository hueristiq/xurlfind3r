// Package wayback implements the Internet Archive CDX provider.
package wayback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const (
	maxPages = 100
	pageSize = 10000
)

// Client is an immutable Wayback client safe for concurrent searches.
type Client struct {
	doer          hqgosource.Doer
	wait          func(context.Context) error
	retryWait     time.Duration
	retryAttempts int
}

// Option customizes a Client during construction.
type Option func(*Client) error

// WithRateLimitWait replaces the default 40-requests-per-minute wait policy.
func WithRateLimitWait(wait func(context.Context) error) Option {
	return func(client *Client) error {
		if wait == nil {
			return errors.New("wayback: nil rate-limit wait function")
		}

		client.wait = wait

		return nil
	}
}

// WithRetryPolicy sets the wait between 429 responses and the number of retries.
func WithRetryPolicy(wait time.Duration, attempts int) Option {
	return func(client *Client) error {
		if wait <= 0 || attempts < 0 {
			return errors.New("wayback: retry wait must be positive and attempts non-negative")
		}

		client.retryWait = wait
		client.retryAttempts = attempts

		return nil
	}
}

// New constructs a client with its own rate limiter.
func New(doer hqgosource.Doer, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.New("wayback: nil HTTP client")
	}

	limiter := hqgolimiter.New(hqgolimiter.WithRate(40, time.Minute))
	client := &Client{
		doer:          doer,
		wait:          limiter.Wait,
		retryWait:     time.Minute,
		retryAttempts: 5,
	}

	for _, apply := range options {
		if apply == nil {
			return nil, errors.New("wayback: nil option")
		}

		if err := apply(client); err != nil {
			return nil, fmt.Errorf("wayback: apply option: %w", err)
		}
	}

	return client, nil
}

// RetryPolicy prevents the injected HTTP client from duplicating the provider's
// own 429 retry loop while retaining its default behavior for other outcomes.
func RetryPolicy(ctx context.Context, req *http.Request, response *http.Response, err error, attempt int) (bool, error) {
	if ctx.Err() == nil && response != nil && response.StatusCode == http.StatusTooManyRequests {
		return false, nil
	}

	return hqgohttp.DefaultRetryPolicy(ctx, req, response, err, attempt) //nolint:wrapcheck // Preserve the shared policy result.
}

// Search streams original URLs and stops on empty or repeated pages, or after
// 100 pages.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("wayback: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("wayback: empty domain")})

			return
		}

		previous := ""
		stopped := false

		for page := range maxPages {
			if err := c.wait(ctx); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("wayback: rate-limit wait: %w", err)})

				return
			}

			data, err := c.getPage(ctx, domain, page)
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			if len(data) <= 1 {
				stopped = true

				break
			}

			signature := fmt.Sprintf("%d:%v:%v", len(data), data[1], data[len(data)-1])
			if signature == previous {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: page contents repeated"})

				stopped = true

				break
			}

			previous = signature

			for _, record := range data[1:] {
				if len(record) < 2 {
					continue
				}

				if !stream.Emit(ctx, results, hqgosource.Result{Value: record[1]}) {
					return
				}
			}
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}

func (c *Client) getPage(ctx context.Context, domain string, page int) ([][]string, error) {
	for attempt := range c.retryAttempts + 1 {
		query := url.Values{
			"url":      {"*." + domain + "/*"},
			"output":   {"json"},
			"collapse": {"urlkey"},
			"fl":       {"timestamp,original,mimetype,statuscode,digest"},
			"pageSize": {strconv.Itoa(pageSize)},
			"page":     {strconv.Itoa(page)},
		}

		response, err := request.Execute(ctx, c.doer, http.MethodGet, "https://web.archive.org/cdx/search/cdx", http.NoBody, query, nil)
		if err != nil {
			return nil, fmt.Errorf("wayback: execute page request: %w", err)
		}

		if response.StatusCode == http.StatusTooManyRequests {
			if response.Body != nil {
				_ = response.Body.Close()
			}

			if attempt == c.retryAttempts {
				break
			}

			if err := stream.Wait(ctx, c.retryWait); err != nil {
				return nil, fmt.Errorf("wayback: rate-limit retry: %w", err)
			}

			continue
		}

		if err := request.Check(response); err != nil {
			if response.Body != nil {
				_ = response.Body.Close()
			}

			return nil, fmt.Errorf("wayback: check page response: %w", err)
		}

		body, err := request.Read(response)
		if err != nil {
			return nil, fmt.Errorf("wayback: read page response: %w", err)
		}

		if strings.TrimSpace(string(body)) == "" {
			return nil, nil
		}

		var data [][]string
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("wayback: decode response: %w", err)
		}

		return data, nil
	}

	return nil, fmt.Errorf("wayback: rate limited after %d attempts", c.retryAttempts+1)
}
