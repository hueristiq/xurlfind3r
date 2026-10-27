package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgohttpheader "github.com/hueristiq/hq-lib-http-go/header"
	hqgohttplink "github.com/hueristiq/hq-lib-http-go/header/link"
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
	name = sources.Github

	return
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
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
		parameters.Set("sort", "indexed")
		parameters.Set("order", "asc")

		for _, query := range searchQueries(domain) {
			parameters.Set("q", fmt.Sprintf("%q", query))

			searchReqURL := searchAPIURL + "?" + parameters.Encode()

			s.Enumerate(ctx, searchReqURL, cfg, results, keysManager)
		}
	}()

	return results
}

func (s *Source) Enumerate(ctx context.Context, searchReqURL string, cfg *sources.Configuration, results chan sources.Result, keysManager *KeysManager) {
	token, index := keysManager.GetCurrentKey()

	for token.RetryAfter > 0 {
		wait, allExceeded := keysManager.WaitForRecovery()
		if !allExceeded {
			token, index = keysManager.GetCurrentKey()

			continue
		}

		if !waitSleep(ctx, wait) {
			return
		}

		token, index = keysManager.GetCurrentKey()
	}

	codeSearchRes, err := cfg.Client.Request().
		URL(searchReqURL).
		SetHeader(hqgohttpheader.Accept.String(), "application/vnd.github.v3.text-match+json").
		SetHeader(hqgohttpheader.Authorization.String(), "token "+token.Value).
		Context(ctx).Do()

	isForbidden := codeSearchRes != nil && codeSearchRes.StatusCode == hqgohttpstatus.Forbidden.Int()

	if err != nil && !isForbidden {
		if codeSearchRes != nil {
			_ = codeSearchRes.Body.Close()
		}

		result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

		results <- result

		return
	}

	if codeSearchRes == nil {
		result := sources.ErrorResult(s.Name(), errors.New("request failed: nil response"))

		results <- result

		return
	}

	ratelimitRemaining := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.XRateLimitRemaining.String()))

	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.RetryAfter.String()))

		if retryAfterSeconds <= 0 {
			retryAfterSeconds = rateLimitResetRemaining(codeSearchRes.Header.Get("X-Ratelimit-Reset"))
		}

		if retryAfterSeconds <= 0 {
			_ = codeSearchRes.Body.Close()

			result := sources.ErrorResult(s.Name(), errors.New("rate limit reached with no recovery hint; try again later"))

			results <- result

			return
		}

		keysManager.SetKeyExceeded(index, retryAfterSeconds)

		_ = codeSearchRes.Body.Close()

		s.Enumerate(ctx, searchReqURL, cfg, results, keysManager)

		return
	}

	if err = sources.CheckResponse(codeSearchRes.Std(), token.Value); err != nil {
		_ = codeSearchRes.Body.Close()

		result := sources.ErrorResult(s.Name(), err)

		results <- result

		return
	}

	var codeSearchResData codeSearchResponse

	if err = json.NewDecoder(codeSearchRes.Body).Decode(&codeSearchResData); err != nil {
		result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

		results <- result

		_ = codeSearchRes.Body.Close()

		return
	}

	_ = codeSearchRes.Body.Close()

	for _, item := range codeSearchResData.Items {
		getRawContentReqURL := strings.ReplaceAll(item.HTMLURL, "https://github.com/", "https://raw.githubusercontent.com/")
		getRawContentReqURL = strings.ReplaceAll(getRawContentReqURL, "/blob/", "/")

		var getRawContentRes *hqgohttp.Response

		getRawContentRes, err = cfg.Client.Request().
			URL(getRawContentReqURL).
			Context(ctx).Do()
		if err != nil {
			if getRawContentRes != nil {
				_ = getRawContentRes.Body.Close()
			}

			result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

			results <- result

			continue
		}

		if getRawContentRes.StatusCode != hqgohttpstatus.OK.Int() {
			_ = getRawContentRes.Body.Close()

			continue
		}

		rawContent, err := io.ReadAll(io.LimitReader(getRawContentRes.Body, maxScanBytes))
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to read response body: %w", err))

			results <- result

			_ = getRawContentRes.Body.Close()

			continue
		}

		_ = getRawContentRes.Body.Close()

		content := string(rawContent)

		sources.SendURLs(results, cfg, s.Name(), sources.ExtractURLsAll(cfg.Extractor, content))
		sources.SendURLs(results, cfg, s.Name(), sources.ExtractCollapsedURLs(cfg.Extractor, content))

		for _, textMatch := range item.TextMatches {
			sources.SendURLs(results, cfg, s.Name(), sources.ExtractURLsAll(cfg.Extractor, textMatch.Fragment))
		}
	}

	links := hqgohttplink.Parse(codeSearchRes.Header.Get(hqgohttpheader.Link.String()))

	for _, link := range links {
		if link.Rel != "next" {
			continue
		}

		nextURL, err := url.Parse(link.URL)
		if err != nil {
			result := sources.ErrorResult(s.Name(), err)

			results <- result

			return
		}

		nextURL.RawQuery = nextURL.Query().Encode()

		s.Enumerate(ctx, nextURL.String(), cfg, results, keysManager)
	}
}

type ManagedKey struct {
	ExceededTime time.Time
	RetryAfter   int64
	Value        string
}

type KeysManager struct {
	mu      sync.Mutex
	current int
	pool    []ManagedKey
}

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

func (m *KeysManager) WaitForRecovery() (wait time.Duration, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.pool {
		key := &m.pool[i]

		if key.RetryAfter <= 0 {
			return 0, false
		}

		remaining := time.Until(key.ExceededTime.Add(time.Duration(key.RetryAfter) * time.Second))

		if remaining <= 0 {
			return 0, false
		}

		if wait == 0 || remaining < wait {
			wait = remaining
		}
	}

	return wait, true
}

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

const (
	searchAPIURL = "https://api.github.com/search/code"

	maxScanBytes = 8 << 20
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}

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

func searchQueries(domain string) (queries []string) {
	queries = []string{
		domain,
		strings.ReplaceAll(domain, ".", `\.`),
		strings.ReplaceAll(domain, ".", "[.]"),
	}

	return
}

func rateLimitResetRemaining(raw string) (remaining int64) {
	reset := cast.ToInt64(raw)

	if reset > 0 {
		remaining = reset - time.Now().Unix()
	}

	return
}

func waitSleep(ctx context.Context, d time.Duration) (completed bool) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
