// Package leakradar implements the Leakradar domain URL provider.
package leakradar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

// Client is an immutable Leakradar client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit API key.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("leakradar: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("leakradar: empty API key")
	}

	return &Client{doer: doer, key: key}, nil
}

type searchResponse struct {
	Items []struct {
		URL string `json:"url"`
	} `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// Search streams URL records using page watermarks to detect repetition and
// stops at the reported total or 100 pages.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("leakradar: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("leakradar: empty domain")})

			return
		}

		previous := ""
		stopped := false

		for page := 1; page <= maxPages; page++ {
			headers := http.Header{"Authorization": {"Bearer " + c.key}}
			query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {"100"}}
			endpoint := "https://api.leakradar.io/search/domain/" + url.PathEscape(domain) + "/urls"

			response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var decoded searchResponse
			if err := request.Decode(response, &decoded); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			signature := fmt.Sprintf("%d|%d", len(decoded.Items), decoded.Total)
			if len(decoded.Items) > 0 {
				signature += "|" + decoded.Items[0].URL + "|" + decoded.Items[len(decoded.Items)-1].URL
			}

			if signature == previous {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: page watermark did not advance"})

				stopped = true

				break
			}

			previous = signature

			for _, item := range decoded.Items {
				if !stream.Emit(ctx, results, hqgosource.Result{Value: item.URL}) {
					return
				}
			}

			if len(decoded.Items) == 0 || decoded.PageSize*decoded.Page >= decoded.Total {
				stopped = true

				break
			}
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}
