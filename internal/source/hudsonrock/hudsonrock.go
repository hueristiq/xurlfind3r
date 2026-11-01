// Package hudsonrock implements the Hudson Rock URLs-by-domain provider.
package hudsonrock

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

// Client is an immutable Hudson Rock client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
}

// New constructs a Hudson Rock client.
func New(doer hqgosource.Doer) (*Client, error) {
	if doer == nil {
		return nil, errors.New("hudsonrock: nil HTTP client")
	}

	return &Client{doer: doer}, nil
}

type searchResponse struct {
	Data struct {
		Employees []struct {
			URL string `json:"url"`
		} `json:"employees_urls"`
		Clients []struct {
			URL string `json:"url"`
		} `json:"clients_urls"`
	} `json:"data"`
}

// Search streams employee and client URLs associated with domain.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("hudsonrock: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("hudsonrock: empty domain")})

			return
		}

		query := url.Values{"domain": {domain}}
		endpoint := "https://cavalier.hudsonrock.com/api/json/v2/osint-tools/urls-by-domain"

		response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, query, nil) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var decoded searchResponse
		if err := request.Decode(response, &decoded); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		for _, record := range decoded.Data.Employees {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: record.URL}) {
				return
			}
		}

		for _, record := range decoded.Data.Clients {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: record.URL}) {
				return
			}
		}
	}()

	return results
}
