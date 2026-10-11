package github

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	hqgohttpheader "github.com/hueristiq/hq-lib-http-go/header"
	hqgohttpheaderutils "github.com/hueristiq/hq-lib-http-go/header/utils"
	hqgohttpstatus "github.com/hueristiq/hq-lib-http-go/status"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type codeSearchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		TextMatches []struct {
			Fragment string `json:"fragment"`
		} `json:"text_matches"`
	} `json:"items"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.GITHUB

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		if len(s.keys) == 0 {
			return
		}

		keysManager := NewKeyManager(s.keys)

		parameters := url.Values{}
		parameters.Set("per_page", "100")
		parameters.Set("q", domain)
		parameters.Set("sort", "created")
		parameters.Set("order", "asc")

		searchReqURL := "https://api.github.com/search/code?" + parameters.Encode()

		s.Enumerate(ctx, searchReqURL, cfg, results, keysManager)
	}()

	return results
}

func (s *Source) Enumerate(ctx context.Context, searchReqURL string, cfg *sources.Configuration, results chan sources.Result, keysManager *KeysManager) {
	token, index := keysManager.GetCurrentKey()

	if token.RetryAfter > 0 {
		if len(keysManager.pool) == 1 {
			retryAfter := time.Duration(token.RetryAfter)*time.Second - time.Since(token.ExceededTime)

			if retryAfter > 0 {
				timer := time.NewTimer(retryAfter)

				select {
				case <-ctx.Done():
					timer.Stop()

					return
				case <-timer.C:
				}
			}
		} else {
			token, index = keysManager.GetCurrentKey()
		}
	}

	codeSearchRes, err := cfg.Client.NewRequest().
		URL(searchReqURL).
		SetHeader(hqgohttpheader.Accept.String(), "application/vnd.github.v3.text-match+json").
		SetHeader(hqgohttpheader.Authorization.String(), "token "+token.Value).
		Do(ctx)

	isForbidden := codeSearchRes != nil && codeSearchRes.StatusCode == hqgohttpstatus.Forbidden.Int()

	if err != nil && !isForbidden {
		if codeSearchRes != nil && codeSearchRes.Body != nil {
			// Close errors on a consumed response body are not actionable.
			_ = codeSearchRes.Body.Close()
		}

		result := sources.Result{
			Type:   sources.ResultError,
			Source: s.Name(),
			Error:  fmt.Errorf("request failed: %w", err),
		}

		results <- result

		return
	}

	ratelimitRemaining := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.XRateLimitRemaining.String()))
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.RetryAfter.String()))

		// Close errors on a consumed response body are not actionable.
		_ = codeSearchRes.Body.Close()

		keysManager.SetKeyExceeded(index, retryAfterSeconds)

		s.Enumerate(ctx, searchReqURL, cfg, results, keysManager)

		return
	}

	var codeSearchResData codeSearchResponse

	if err = json.NewDecoder(codeSearchRes.Body).Decode(&codeSearchResData); err != nil {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: s.Name(),
			Error:  fmt.Errorf("failed to parse JSON response: %w", err),
		}

		results <- result

		// Close errors on a consumed response body are not actionable.
		_ = codeSearchRes.Body.Close()

		return
	}

	// Close errors on a consumed response body are not actionable.
	_ = codeSearchRes.Body.Close()

	for _, item := range codeSearchResData.Items {
		getRawContentReqURL := strings.ReplaceAll(item.HTMLURL, "https://github.com/", "https://raw.githubusercontent.com/")
		getRawContentReqURL = strings.ReplaceAll(getRawContentReqURL, "/blob/", "/")

		var getRawContentRes *http.Response

		getRawContentRes, err = cfg.Client.NewRequest().
			URL(getRawContentReqURL).
			Do(ctx)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("request failed: %w", err),
			}

			results <- result

			continue
		}

		if getRawContentRes.StatusCode != hqgohttpstatus.OK.Int() {
			// Close errors on a consumed response body are not actionable.
			_ = getRawContentRes.Body.Close()

			continue
		}

		scanner := bufio.NewScanner(getRawContentRes.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			URLs := sources.ExtractURLs(cfg.Extractor, line)

			for _, URL := range URLs {
				var valid bool

				if URL, valid = cfg.Validator(URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result
			}
		}

		if err = scanner.Err(); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to read response body: %w", err),
			}

			results <- result

			// Close errors on a consumed response body are not actionable.
			_ = getRawContentRes.Body.Close()

			continue
		}

		// Close errors on a consumed response body are not actionable.
		_ = getRawContentRes.Body.Close()

		for _, textMatch := range item.TextMatches {
			URLs := sources.ExtractURLs(cfg.Extractor, textMatch.Fragment)

			for _, URL := range URLs {
				var valid bool

				if URL, valid = cfg.Validator(URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result
			}
		}
	}

	links := hqgohttpheaderutils.Parse(codeSearchRes.Header.Get(hqgohttpheader.Link.String()))

	for _, link := range links {
		if link.Rel == "next" {
			nextURL, err := url.QueryUnescape(link.URL)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			s.Enumerate(ctx, nextURL, cfg, results, keysManager)
		}
	}
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

// ManagedKey is a GitHub API token tracked by [KeysManager].
//
// Fields:
//   - ExceededTime (time.Time): when the token was last rate-limited.
//   - RetryAfter (int64): seconds to wait before reusing the token.
//   - Value (string): the token itself.
type ManagedKey struct {
	ExceededTime time.Time
	RetryAfter   int64
	Value        string
}

// KeysManager rotates a pool of GitHub API tokens, tracking which ones
// are currently rate-limited.
//
// Fields:
//   - mu (sync.Mutex): guards the pool and cursor.
//   - current (int): the index of the next token to hand out.
//   - pool ([]ManagedKey): the tokens under management.
type KeysManager struct {
	mu      sync.Mutex
	current int
	pool    []ManagedKey
}

// NewKeyManager returns a KeysManager over the given API tokens.
//
// Parameters:
//   - keys ([]string): the GitHub API tokens.
//
// Returns:
//   - manager (*KeysManager): the key manager.
func NewKeyManager(keys []string) (manager *KeysManager) {
	pool := make([]ManagedKey, len(keys))

	for i, key := range keys {
		pool[i] = ManagedKey{Value: key}
	}

	manager = &KeysManager{
		pool: pool,
	}

	return
}

// GetCurrentKey returns the next token and the index it occupies in the
// pool. Tokens whose rate-limit window has elapsed are reactivated first.
//
// Returns:
//   - key (*ManagedKey): the token to use for the next request.
//   - index (int): the token's index in the pool; pass it to
//     [KeysManager.SetKeyExceeded] if the request is rate-limited.
func (m *KeysManager) GetCurrentKey() (key *ManagedKey, index int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.pool {
		poolKey := &m.pool[i]

		if poolKey.RetryAfter > 0 && time.Since(poolKey.ExceededTime) > time.Duration(poolKey.RetryAfter)*time.Second {
			poolKey.ExceededTime = time.Time{}
			poolKey.RetryAfter = 0
		}
	}

	if m.current >= len(m.pool) {
		m.current %= len(m.pool)
	}

	index = m.current
	key = &m.pool[index]

	m.current++

	return
}

// SetKeyExceeded marks the token at index as rate-limited for the given
// number of seconds.
//
// Parameters:
//   - index (int): the token's index, as returned by [KeysManager.GetCurrentKey].
//   - retryAfter (int64): seconds to wait before reusing the token.
func (m *KeysManager) SetKeyExceeded(index int, retryAfter int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index >= len(m.pool) {
		return
	}

	key := &m.pool[index]

	if key.RetryAfter == 0 {
		key.ExceededTime = time.Now()
		key.RetryAfter = retryAfter
	}
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
