// Package arquivo implements the Arquivo.pt CDX provider.
package arquivo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxScanTokenSize = 1 << 20

// Client is an immutable Arquivo.pt client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
}

// New constructs an Arquivo.pt client.
func New(doer hqgosource.Doer) (*Client, error) {
	if doer == nil {
		return nil, errors.New("arquivo: nil HTTP client")
	}

	return &Client{doer: doer}, nil
}

// Search streams URL records from the Arquivo.pt CDX endpoint. Blank and
// malformed lines are skipped, matching the provider's tolerant record stream.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("arquivo: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("arquivo: empty domain")})

			return
		}

		query := url.Values{
			"url":      {"*." + domain + "/*"},
			"output":   {"json"},
			"fl":       {"url"},
			"collapse": {"urlkey"},
			"limit":    {"100000"},
		}

		response, err := request.Do(ctx, c.doer, http.MethodGet, "https://arquivo.pt/wayback/cdx", http.NoBody, query, nil) //nolint:bodyclose // scan closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		c.scan(ctx, response, results)
	}()

	return results
}

func (c *Client) scan(ctx context.Context, response *http.Response, results chan<- hqgosource.Result) {
	if response == nil || response.Body == nil {
		stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("arquivo: empty response body")})

		return
	}

	defer func() {
		_ = response.Body.Close()
	}()

	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var record struct {
			URL string `json:"url"`
		}

		if err := json.Unmarshal(line, &record); err != nil || record.URL == "" {
			continue
		}

		if !stream.Emit(ctx, results, hqgosource.Result{Value: record.URL}) {
			return
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		stream.Emit(ctx, results, hqgosource.Result{Err: err})
	}
}
