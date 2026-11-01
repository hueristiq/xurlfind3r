// Package bevigil implements the Be Vigil subdomain and URL provider.
package bevigil

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

// Target selects the Be Vigil endpoint queried by Search.
type Target int

const (
	// Domains selects the subdomains endpoint.
	Domains Target = iota + 1
	// URLs selects the URLs endpoint.
	URLs
)

// Client is an immutable Be Vigil client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit API key.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("bevigil: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("bevigil: empty API key")
	}

	return &Client{doer: doer, key: key}, nil
}

type searchResponse struct {
	Subdomains []string `json:"subdomains"`
	URLs       []string `json:"urls"`
}

// Search streams values from the selected Be Vigil endpoint.
func (c *Client) Search(ctx context.Context, domain string, target Target) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("bevigil: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("bevigil: empty domain")})

			return
		}

		var endpoint string

		switch target {
		case Domains:
			endpoint = "subdomains"
		case URLs:
			endpoint = "urls"
		default:
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("bevigil: unsupported target")})

			return
		}

		headers := http.Header{"X-Access-Token": {c.key}}
		endpoint = "https://osint.bevigil.com/api/" + url.PathEscape(domain) + "/" + endpoint + "/"

		response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, nil, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var decoded searchResponse
		if err := request.Decode(response, &decoded); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		values := decoded.Subdomains
		if target == URLs {
			values = decoded.URLs
		}

		for _, value := range values {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: value}) {
				return
			}
		}
	}()

	return results
}
