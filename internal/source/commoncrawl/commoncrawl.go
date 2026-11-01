// Package commoncrawl implements the Common Crawl index and CDX provider.
package commoncrawl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const (
	maxScanTokenSize = 1 << 20
	maxPages         = 100
	maxYearsBack     = 5
	maxEmptyPages    = 3
)

// Client is an immutable Common Crawl client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	wait func(context.Context) error
}

// Option customizes a Client during construction.
type Option func(*Client) error

// WithRateLimitWait replaces the default 30-requests-per-minute wait policy.
// The function must honor context cancellation.
func WithRateLimitWait(wait func(context.Context) error) Option {
	return func(client *Client) error {
		if wait == nil {
			return errors.New("commoncrawl: nil rate-limit wait function")
		}

		client.wait = wait

		return nil
	}
}

// New constructs a client with its own rate limiter.
func New(doer hqgosource.Doer, options ...Option) (*Client, error) {
	if doer == nil {
		return nil, errors.New("commoncrawl: nil HTTP client")
	}

	limiter := hqgolimiter.New(hqgolimiter.WithRate(30, time.Minute))
	client := &Client{doer: doer, wait: limiter.Wait}

	for _, apply := range options {
		if apply == nil {
			return nil, errors.New("commoncrawl: nil option")
		}

		if err := apply(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

type index struct {
	ID     string `json:"id"`
	CDXAPI string `json:"cdx-api"`
}

type pagination struct {
	Pages uint `json:"pages"`
}

type record struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// Search streams URL records from the most recent index in each of the last
// five years. Each index is limited to 100 pages, and three consecutive empty
// pages end that index early.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("commoncrawl: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("commoncrawl: empty domain")})

			return
		}

		response, err := request.Do(ctx, c.doer, http.MethodGet, "https://index.commoncrawl.org/collinfo.json", http.NoBody, nil, nil) //nolint:bodyclose // request.Decode closes the body.
		if err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		var indexes []index
		if err := request.Decode(response, &indexes); err != nil {
			stream.Emit(ctx, results, hqgosource.Result{Err: err})

			return
		}

		for _, endpoint := range searchIndexes(indexes, time.Now().Year()) {
			c.searchIndex(ctx, domain, endpoint, results)
		}
	}()

	return results
}

func searchIndexes(indexes []index, year int) []string {
	endpoints := make([]string, 0, maxYearsBack)

	for offset := range maxYearsBack {
		year := strconv.Itoa(year - offset)

		for _, candidate := range indexes {
			if candidate.CDXAPI != "" && strings.Contains(candidate.ID, year) {
				endpoints = append(endpoints, candidate.CDXAPI)

				break
			}
		}
	}

	return endpoints
}

func (c *Client) searchIndex(ctx context.Context, domain, endpoint string, results chan<- hqgosource.Result) {
	query := url.Values{
		"url":      {"*." + domain},
		"output":   {"json"},
		"fl":       {"url"},
		"collapse": {"urlkey"},
	}

	paginationQuery := cloneQuery(query)
	paginationQuery.Set("showNumPages", "true")

	response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, paginationQuery, nil) //nolint:bodyclose // request.Decode closes the body.
	if err != nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: err}) {
			return
		}

		return
	}

	var pages pagination
	if err := request.Decode(response, &pages); err != nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: err}) {
			return
		}

		return
	}

	pageCount := pages.Pages
	if pageCount > maxPages {
		pageCount = maxPages

		if !stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)}) {
			return
		}
	}

	emptyPages := 0

	for page := range pageCount {
		if err := c.wait(ctx); err != nil {
			if !stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("commoncrawl: rate-limit wait: %w", err)}) {
				return
			}

			return
		}

		pageQuery := cloneQuery(query)
		pageQuery.Set("page", strconv.FormatUint(uint64(page), 10))

		response, err := request.Do(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, pageQuery, nil) //nolint:bodyclose // scanPage closes the body.
		if err != nil {
			if !stream.Emit(ctx, results, hqgosource.Result{Err: err}) {
				return
			}

			continue
		}

		records, proceed := c.scanPage(ctx, response, results)
		if !proceed {
			return
		}

		if records == 0 {
			emptyPages++
		} else {
			emptyPages = 0
		}

		if emptyPages >= maxEmptyPages {
			return
		}
	}
}

func (c *Client) scanPage(ctx context.Context, response *http.Response, results chan<- hqgosource.Result) (records int, proceed bool) {
	if response == nil || response.Body == nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("commoncrawl: empty response body")}) {
			return 0, false
		}

		return 0, true
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

		var item record
		if err := json.Unmarshal(line, &item); err != nil {
			if !stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("commoncrawl: decode record: %w", err)}) {
				return records, false
			}

			continue
		}

		if item.Error != "" {
			if !stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("commoncrawl: domain error: %s", item.Error)}) {
				return records, false
			}

			continue
		}

		if item.URL == "" {
			continue
		}

		records++

		if !stream.Emit(ctx, results, hqgosource.Result{Value: item.URL}) {
			return records, false
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: fmt.Errorf("commoncrawl: read response: %w", err)}) {
			return records, false
		}
	}

	return records, true
}

func cloneQuery(query url.Values) url.Values {
	clone := make(url.Values, len(query)+1)
	for name, values := range query {
		clone[name] = append([]string(nil), values...)
	}

	return clone
}
