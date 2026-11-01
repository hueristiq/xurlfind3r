// Package otx implements AlienVault Open Threat Exchange providers.
package otx

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

// Client is an immutable OTX client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit API key.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("otx: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("otx: empty API key")
	}

	return &Client{doer: doer, key: key}, nil
}

// PassiveDNS streams hostnames from the domain passive-DNS endpoint.
func (c *Client) PassiveDNS(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("otx: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("otx: empty domain")})

			return
		}

		headers := http.Header{"X-Otx-Api-Key": {c.key}}
		endpoint := "https://otx.alienvault.com/api/v1/indicators/domain/" + url.PathEscape(domain) + "/passive_dns"

		response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, nil, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var decoded struct {
			Detail     string `json:"detail"`
			Error      string `json:"error"`
			PassiveDNS []struct {
				Hostname string `json:"hostname"`
			} `json:"passive_dns"`
		}

		if err := request.Decode(response, &decoded); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		if decoded.Error != "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("otx: domain error: %s, %s", decoded.Detail, decoded.Error)})

			return
		}

		for _, record := range decoded.PassiveDNS {
			if !stream.Emit(ctx, results, hqgosource.Result{Value: record.Hostname}) {
				return
			}
		}
	}()

	return results
}

// URLs streams URL records using page watermarks and at most 100 pages.
func (c *Client) URLs(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("otx: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("otx: empty domain")})

			return
		}

		previous := ""
		stopped := false

		for page := 1; page <= maxPages; page++ {
			headers := http.Header{"X-Otx-Api-Key": {c.key}}
			query := url.Values{"limit": {"100"}, "page": {strconv.Itoa(page)}}
			endpoint := "https://otx.alienvault.com/api/v1/indicators/domain/" + url.PathEscape(domain) + "/url_list"

			response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var decoded struct {
				URLList []struct {
					URL string `json:"url"`
				} `json:"url_list"`
				ActualSize int  `json:"actual_size"`
				HasNext    bool `json:"has_next"`
			}

			if err := request.Decode(response, &decoded); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			signature := fmt.Sprintf("%d|%d", len(decoded.URLList), decoded.ActualSize)
			if len(decoded.URLList) > 0 {
				signature += "|" + decoded.URLList[0].URL + "|" + decoded.URLList[len(decoded.URLList)-1].URL
			}

			if signature == previous {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: page watermark did not advance"})

				stopped = true

				break
			}

			previous = signature

			for _, item := range decoded.URLList {
				if !stream.Emit(ctx, results, hqgosource.Result{Value: item.URL}) {
					return
				}
			}

			if !decoded.HasNext {
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
