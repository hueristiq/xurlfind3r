// Package virustotal implements VirusTotal domain and URL providers.
package virustotal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

// Client is an immutable VirusTotal client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
	wait func(context.Context) error
}

// Option customizes a Client during construction.
type Option func(*Client) error

// WithRateLimitWait replaces the default four-requests-per-minute wait policy.
func WithRateLimitWait(wait func(context.Context) error) Option {
	return func(client *Client) error {
		if wait == nil {
			return errors.New("virustotal: nil rate-limit wait function")
		}

		client.wait = wait

		return nil
	}
}

// New constructs a client with its own rate limiter and an explicit API key.
func New(doer hqgosource.Doer, key string, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.New("virustotal: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("virustotal: empty API key")
	}

	limiter := hqgolimiter.New(hqgolimiter.WithRate(4, time.Minute))
	client := &Client{doer: doer, key: key, wait: limiter.Wait}

	for _, apply := range options {
		if apply == nil {
			return nil, errors.New("virustotal: nil option")
		}

		if err := apply(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

type subdomainsResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Meta struct {
		Cursor string `json:"cursor"`
	} `json:"meta"`
}

// Subdomains streams v3 subdomains using cursor pagination.
func (c *Client) Subdomains(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("virustotal: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("virustotal: empty domain")})

			return
		}

		cursor := ""
		stopped := false

		for range maxPages {
			if err := c.wait(ctx); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("virustotal: rate-limit wait: %w", err)})

				return
			}

			query := url.Values{"limit": {"40"}}
			if cursor != "" {
				query.Set("cursor", cursor)
			}

			headers := http.Header{"X-Apikey": {c.key}}
			endpoint := "https://www.virustotal.com/api/v3/domains/" + url.PathEscape(domain) + "/subdomains"

			response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var decoded subdomainsResponse
			if err := request.Decode(response, &decoded); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			if decoded.Error.Message != "" {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("virustotal: domain error: %s, %s", decoded.Error.Code, decoded.Error.Message)})

				return
			}

			for _, record := range decoded.Data {
				if !stream.Emit(ctx, results, hqgosource.Result{Value: record.ID}) {
					return
				}
			}

			if decoded.Meta.Cursor == "" {
				stopped = true

				break
			}

			if decoded.Meta.Cursor == cursor {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: cursor did not advance"})

				stopped = true

				break
			}

			cursor = decoded.Meta.Cursor
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}

type domainReportResponse struct {
	DetectedURLs []struct {
		URL string `json:"url"`
	} `json:"detected_urls"`
	Subdomains     []string `json:"subdomains"`
	UndetectedURLs [][]any  `json:"undetected_urls"`
}

// DomainReport streams detected URLs, subdomains, and undetected URLs from the
// v2 domain report endpoint.
func (c *Client) DomainReport(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("virustotal: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("virustotal: empty domain")})

			return
		}

		if err := c.wait(ctx); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("virustotal: rate-limit wait: %w", err)})

			return
		}

		query := url.Values{"apikey": {c.key}, "domain": {domain}}

		response, err := request.Do(ctx, c.doer, http.MethodGet, "https://www.virustotal.com/vtapi/v2/domain/report", http.NoBody, query, nil, c.key) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var decoded domainReportResponse
		if err := request.Decode(response, &decoded); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		for _, record := range decoded.DetectedURLs {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: record.URL}) {
				return
			}
		}

		for _, subdomain := range decoded.Subdomains {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: subdomain}) {
				return
			}
		}

		for _, record := range decoded.UndetectedURLs {
			if len(record) == 0 {
				continue
			}

			value, ok := record[0].(string)
			if !ok {
				continue
			}

			if !stream.Emit(ctx, results, hqgosource.Result{Value: value}) {
				return
			}
		}
	}()

	return results
}
