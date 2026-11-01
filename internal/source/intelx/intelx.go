// Package intelx implements the Intelligence X phonebook search provider.
package intelx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

// Target selects the phonebook record type returned by Search.
type Target int

const (
	// Domains selects domain records.
	Domains Target = 1
	// URLs selects URL records.
	URLs Target = 3
)

const (
	defaultPollInterval  = 10 * time.Second
	defaultPollAttempts  = 30
	defaultSearchTimeout = 20
)

// Client is an immutable Intelligence X client safe for concurrent searches.
type Client struct {
	doer         hqgosource.Doer
	baseURL      string
	key          string
	pollInterval time.Duration
	pollAttempts int
}

// Option customizes a Client during construction.
type Option func(*Client) error

// WithPollPolicy sets the delay and maximum number of result polls.
func WithPollPolicy(interval time.Duration, attempts int) Option {
	return func(client *Client) error {
		if interval <= 0 || attempts <= 0 {
			return errors.New("intelx: poll interval and attempts must be positive")
		}

		client.pollInterval = interval
		client.pollAttempts = attempts

		return nil
	}
}

// New validates a host:key credential and constructs a client.
func New(doer hqgosource.Doer, credential string, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.New("intelx: nil HTTP client")
	}

	separator := strings.LastIndex(credential, ":")
	if separator <= 0 || separator == len(credential)-1 {
		return nil, errors.New("intelx: credential must use host:key format")
	}

	host := strings.TrimSpace(credential[:separator])

	key := credential[separator+1:]
	if host == "" || key == "" || strings.Contains(host, "://") || strings.Contains(host, "/") {
		return nil, errors.New("intelx: credential must contain a valid host and non-empty key")
	}

	client := &Client{
		doer:         doer,
		baseURL:      (&url.URL{Scheme: "https", Host: host}).String(),
		key:          key,
		pollInterval: defaultPollInterval,
		pollAttempts: defaultPollAttempts,
	}

	for _, apply := range options {
		if apply == nil {
			return nil, errors.New("intelx: nil option")
		}

		if err := apply(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

type searchRequest struct {
	Term       string `json:"term"`
	MaxResults int    `json:"maxresults"`
	Media      int    `json:"media"`
	Target     Target `json:"target"`
	Timeout    int    `json:"timeout"`
}

type searchResponse struct {
	ID string `json:"id"`
}

type resultsResponse struct {
	Selectors []struct {
		Value string `json:"selectorvalue"`
	} `json:"selectors"`
	Status int `json:"status"`
}

// Search starts a phonebook search and streams unique selectors. The channel
// closes on completion or context cancellation. Polling continues while the
// provider reports status 0 or 3.
func (c *Client) Search(ctx context.Context, domain string, target Target) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("intelx: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("intelx: empty domain")})

			return
		}

		if target != Domains && target != URLs {
			stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("intelx: unsupported target %d", target)})

			return
		}

		term := domain
		if target == URLs {
			term = "*" + domain
		}

		body, err := json.Marshal(searchRequest{
			Term:       term,
			MaxResults: 100000,
			Target:     target,
			Timeout:    defaultSearchTimeout,
		})
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("intelx: encode search: %w", err)})

			return
		}

		query := url.Values{"k": {c.key}}
		headers := http.Header{"Content-Type": {"application/json"}}

		response, err := request.Do(ctx, c.doer, http.MethodPost, c.baseURL+"/phonebook/search", bytes.NewReader(body), query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var search searchResponse
		if err := request.Decode(response, &search); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		if search.ID == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("intelx: search returned no ID")})

			return
		}

		status := 0
		seen := make(map[string]struct{})

		for attempt := range c.pollAttempts {
			if attempt > 0 {
				if status != 0 && status != 3 {
					return
				}

				if err := stream.Wait(ctx, c.pollInterval); err != nil {
					return
				}
			}

			query := url.Values{
				"k":     {c.key},
				"id":    {search.ID},
				"limit": {"10000"},
			}

			response, err := request.Do(ctx, c.doer, http.MethodGet, c.baseURL+"/phonebook/search/result", http.NoBody, query, nil, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			var page resultsResponse
			if err := request.Decode(response, &page); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			status = page.Status

			for _, selector := range page.Selectors {
				if _, duplicate := seen[selector.Value]; duplicate {
					continue
				}

				seen[selector.Value] = struct{}{}

				if !stream.Emit(ctx, results, hqgosource.Result{Value: selector.Value}) {
					return
				}
			}
		}

		if status == 0 || status == 3 {
			stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("intelx: result polling did not finish after %d attempts", c.pollAttempts)})
		}
	}()

	return results
}
