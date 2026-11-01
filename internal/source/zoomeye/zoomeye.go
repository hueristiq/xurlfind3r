// Package zoomeye implements the ZoomEye v2 search provider.
package zoomeye

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

// Target selects the values emitted for each ZoomEye record.
type Target int

const (
	// Domains emits both domain and URL fields for subdomain extraction.
	Domains Target = iota + 1
	// URLs emits only URL fields.
	URLs
)

// Client is an immutable ZoomEye client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit API key.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("zoomeye: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("zoomeye: empty API key")
	}

	return &Client{doer: doer, key: key}, nil
}

type searchRequest struct {
	Query    string `json:"qbase64"`
	Page     int    `json:"page"`
	PageSize int    `json:"pagesize"`
	SubType  string `json:"sub_type"`
}

type searchResponse struct {
	Total int `json:"total"`
	Data  []struct {
		URL    string `json:"url"`
		Domain string `json:"domain"`
	} `json:"data"`
}

// Search streams records using 100-item pages and stops at the reported total
// or 100 pages.
func (c *Client) Search(ctx context.Context, domain string, target Target) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("zoomeye: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("zoomeye: empty domain")})

			return
		}

		if target != Domains && target != URLs {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("zoomeye: unsupported target")})

			return
		}

		processed := 0
		stopped := false

		for page := 1; page <= maxPages; page++ {
			body, err := json.Marshal(searchRequest{
				Query:    base64.StdEncoding.EncodeToString([]byte("*." + domain)),
				Page:     page,
				PageSize: 100,
				SubType:  "web",
			})
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("zoomeye: encode request: %w", err)})

				return
			}

			headers := http.Header{"Api-Key": {c.key}, "Content-Type": {"application/json"}}

			response, err := request.Do(ctx, c.doer, http.MethodPost, "https://api.zoomeye.ai/v2/search", bytes.NewReader(body), nil, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var decoded searchResponse
			if err := request.Decode(response, &decoded); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			if len(decoded.Data) == 0 {
				stopped = true

				break
			}

			for _, record := range decoded.Data {
				processed++

				if target == Domains && record.Domain != "" {
					if !stream.Emit(ctx, results, hqgosource.Result{Value: record.Domain}) {
						return
					}
				}

				if record.URL != "" {
					if !stream.Emit(ctx, results, hqgosource.Result{Value: record.URL}) {
						return
					}
				}
			}

			if processed >= decoded.Total {
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
