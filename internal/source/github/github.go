// Package github implements GitHub code search with key rotation and content retrieval.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	hqgohttplink "github.com/hueristiq/hq-lib-http-go/header/link"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const (
	searchAPIURL = "https://api.github.com/search/code"
	maxPages     = 100

	// Content identifies a complete raw file.
	Content = "content"
	// Match identifies a code-search text-match fragment.
	Match = "match"
)

// Query describes one GitHub code search. Empty Sort selects created and empty
// Order selects ascending.
type Query struct {
	Term  string
	Sort  string
	Order string
}

// Client is a GitHub client with an isolated, concurrency-safe key pool.
type Client struct {
	doer hqgosource.Doer
	keys *keyManager
}

// New constructs a client with one or more API keys.
func New(doer hqgosource.Doer, keys []string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("github: nil HTTP client")
	}

	if len(keys) == 0 {
		return nil, errors.New("github: no API keys configured")
	}

	for _, key := range keys {
		if key == "" {
			return nil, errors.New("github: empty API key")
		}
	}

	return &Client{doer: doer, keys: newKeyManager(keys)}, nil
}

type searchResponse struct {
	Items []struct {
		HTMLURL     string `json:"html_url"`
		TextMatches []struct {
			Fragment string `json:"fragment"`
		} `json:"text_matches"`
	} `json:"items"`
}

// Search executes query and streams raw content and text matches. Link pages
// are followed iteratively, with repeated-link and 100-page limits.
func (c *Client) Search(ctx context.Context, query Query) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("github: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(query.Term) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("github: empty query")})

			return
		}

		if query.Sort == "" {
			query.Sort = "created"
		}

		if query.Order == "" {
			query.Order = "asc"
		}

		parameters := url.Values{
			"per_page": {"100"},
			"q":        {query.Term},
			"sort":     {query.Sort},
			"order":    {query.Order},
		}

		endpoint := searchAPIURL + "?" + parameters.Encode()
		seen := map[string]struct{}{}
		stopped := false

		for range maxPages {
			if _, duplicate := seen[endpoint]; duplicate {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: link did not advance"})

				stopped = true

				break
			}

			seen[endpoint] = struct{}{}

			next, err := c.searchPage(ctx, endpoint, results)
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			if next == "" {
				stopped = true

				break
			}

			endpoint = next
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}

func (c *Client) searchPage(ctx context.Context, endpoint string, results chan<- hqgosource.Result) (string, error) {
	recoveries := 0

	for {
		key, index, ok := c.keys.next()
		if !ok {
			if recoveries >= c.keys.len() {
				return "", errors.New("github: API keys remain rate limited")
			}

			recoveries++

			wait, required := c.keys.recovery()
			if !required {
				continue
			}

			if err := stream.Wait(ctx, wait); err != nil {
				return "", fmt.Errorf("github: wait for key recovery: %w", err)
			}

			continue
		}

		headers := http.Header{
			"Accept":        {"application/vnd.github.v3.text-match+json"},
			"Authorization": {"token " + key},
		}

		response, err := request.Execute(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, nil, headers, key)
		if err != nil && (response == nil || response.StatusCode != http.StatusForbidden) {
			return "", fmt.Errorf("github: execute search request: %w", err)
		}

		if response.StatusCode == http.StatusForbidden && response.Header.Get("X-Ratelimit-Remaining") == "0" {
			retryAfter := recoverySeconds(response.Header)
			if retryAfter <= 0 {
				if response.Body != nil {
					_ = response.Body.Close()
				}

				return "", errors.New("github: rate limit reached with no recovery hint")
			}

			c.keys.exceeded(index, retryAfter)

			if response.Body != nil {
				_ = response.Body.Close()
			}

			continue
		}

		if err != nil {
			if response.Body != nil {
				_ = response.Body.Close()
			}

			return "", fmt.Errorf("github: execute search request: %w", err)
		}

		if err := request.Check(response, key); err != nil {
			if response.Body != nil {
				_ = response.Body.Close()
			}

			return "", fmt.Errorf("github: check search response: %w", err)
		}

		next, err := nextLink(response.Header.Get("Link"))
		if err != nil {
			return "", fmt.Errorf("github: follow next link: %w", err)
		}

		var decoded searchResponse
		if err := request.Decode(response, &decoded); err != nil {
			return "", fmt.Errorf("github: decode search response: %w", err)
		}

		for _, item := range decoded.Items {
			found, proceed := c.sendContent(ctx, item.HTMLURL, results)
			if !proceed {
				return "", nil
			}

			if !found {
				continue
			}

			for _, match := range item.TextMatches {
				if !stream.Emit(ctx, results, hqgosource.Result{Value: match.Fragment, Kind: Match}) {
					return "", nil
				}
			}
		}

		return next, nil
	}
}

func (c *Client) sendContent(ctx context.Context, htmlURL string, results chan<- hqgosource.Result) (found, proceed bool) {
	endpoint := strings.ReplaceAll(htmlURL, "https://github.com/", "https://raw.githubusercontent.com/")
	endpoint = strings.ReplaceAll(endpoint, "/blob/", "/")

	response, err := request.Execute(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, nil, nil)
	if err != nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: err}) {
			return false, false
		}

		return false, true
	}

	if response.StatusCode != http.StatusOK {
		if response.Body != nil {
			_ = response.Body.Close()
		}

		return false, true
	}

	body, err := request.Read(response)
	if err != nil {
		if !stream.Emit(ctx, results, hqgosource.Result{Err: err}) {
			return false, false
		}

		return false, true
	}

	if len(body) == 0 {
		return true, true
	}

	return true, stream.Emit(ctx, results, hqgosource.Result{Value: string(body), Kind: Content})
}

func nextLink(header string) (string, error) {
	for _, link := range hqgohttplink.Parse(header) {
		if link.Rel != "next" {
			continue
		}

		parsed, err := url.Parse(link.URL)
		if err != nil {
			return "", fmt.Errorf("github: parse next link: %w", err)
		}

		parsed.RawQuery = parsed.Query().Encode()

		return parsed.String(), nil
	}

	return "", nil
}

func recoverySeconds(headers http.Header) int64 {
	if value := headers.Get("Retry-After"); value != "" {
		seconds, _ := strconv.ParseInt(value, 10, 64)
		if seconds > 0 {
			return seconds
		}
	}

	reset, _ := strconv.ParseInt(headers.Get("X-Ratelimit-Reset"), 10, 64)
	if reset > 0 {
		return reset - time.Now().Unix()
	}

	return 0
}

type managedKey struct {
	exceeded time.Time
	after    int64
	value    string
}

type keyManager struct {
	mu      sync.Mutex
	current int
	pool    []managedKey
}

func newKeyManager(keys []string) *keyManager {
	pool := make([]managedKey, len(keys))
	for index, key := range keys {
		pool[index].value = key
	}

	return &keyManager{pool: pool}
}

func (m *keyManager) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.pool)
}

func (m *keyManager) next() (value string, index int, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for index := range m.pool {
		key := &m.pool[index]
		if key.after > 0 && now.Sub(key.exceeded) >= time.Duration(key.after)*time.Second {
			key.exceeded = time.Time{}
			key.after = 0
		}
	}

	for range m.pool {
		index := m.current % len(m.pool)
		m.current++

		key := &m.pool[index]
		if key.after == 0 {
			return key.value, index, true
		}
	}

	return "", 0, false
}

func (m *keyManager) recovery() (time.Duration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var earliest time.Duration

	for index := range m.pool {
		key := &m.pool[index]
		if key.after <= 0 {
			continue
		}

		remaining := time.Until(key.exceeded.Add(time.Duration(key.after) * time.Second))
		if remaining <= 0 {
			return 0, false
		}

		if earliest == 0 || remaining < earliest {
			earliest = remaining
		}
	}

	return earliest, earliest > 0
}

func (m *keyManager) exceeded(index int, after int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index < 0 || index >= len(m.pool) || m.pool[index].after != 0 {
		return
	}

	m.pool[index].exceeded = time.Now()
	m.pool[index].after = after
}
