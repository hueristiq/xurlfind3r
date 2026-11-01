// Package urlscan implements the urlscan.io search provider.
package urlscan

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

// Target selects the value emitted for each urlscan.io result.
type Target int

const (
	// Domains emits the result page's domain.
	Domains Target = iota + 1
	// URLs emits the result page's URL.
	URLs
)

// Client is an immutable urlscan.io client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit API key.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("urlscan: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("urlscan: empty API key")
	}

	return &Client{doer: doer, key: key}, nil
}

type searchResponse struct {
	Results []struct {
		Page struct {
			Domain string `json:"domain"`
			URL    string `json:"url"`
		} `json:"page"`
		Sort []any `json:"sort"`
	} `json:"results"`
	Status  int  `json:"status"`
	HasMore bool `json:"has_more"`
}

// Search streams results using search_after cursors and stops on repeated
// cursors, empty pages, or 100 pages.
func (c *Client) Search(ctx context.Context, domain string, target Target) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("urlscan: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("urlscan: empty domain")})

			return
		}

		if target != Domains && target != URLs {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("urlscan: unsupported target")})

			return
		}

		after := ""
		stopped := false

		for range maxPages {
			query := url.Values{"q": {"domain:" + domain}, "size": {"10000"}}
			if after != "" {
				query.Set("search_after", after)
			}

			headers := http.Header{"Accept": {"application/json"}, "Api-Key": {c.key}}

			response, err := request.Do(ctx, c.doer, http.MethodGet, "https://urlscan.io/api/v1/search", http.NoBody, query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var decoded searchResponse
			if err := request.Decode(response, &decoded); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			if decoded.Status == http.StatusTooManyRequests {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("urlscan: request failed with status %d", decoded.Status)})

				return
			}

			for _, item := range decoded.Results {
				value := item.Page.Domain
				if target == URLs {
					value = item.Page.URL
				}

				if !stream.Emit(ctx, results, hqgosource.Result{Value: value}) {
					return
				}
			}

			if !decoded.HasMore || len(decoded.Results) == 0 {
				stopped = true

				break
			}

			last := decoded.Results[len(decoded.Results)-1]
			if len(last.Sort) == 0 {
				stopped = true

				break
			}

			sortValues := make([]string, 0, len(last.Sort))
			for _, value := range last.Sort {
				sortValues = append(sortValues, fmt.Sprint(value))
			}

			nextAfter := strings.Join(sortValues, ",")
			if nextAfter == "" || nextAfter == after {
				if nextAfter == after {
					stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: search cursor did not advance"})
				}

				stopped = true

				break
			}

			after = nextAfter
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}
