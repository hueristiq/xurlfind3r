package github

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgohttpheader "github.com/hueristiq/hq-go-http/header"
	hqgohttpheaderutils "github.com/hueristiq/hq-go-http/header/utils"
	hqgohttpstatus "github.com/hueristiq/hq-go-http/status"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
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
	keys        sources.Keys
	keysManager *KeysManager
}

func (s *Source) Name() (name string) {
	name = sources.GITHUB

	return
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		if len(s.keys) == 0 {
			return
		}

		s.keysManager = NewKeyManager(s.keys)

		searchReqURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%q&sort=created&order=asc", domain)

		s.Enumerate(searchReqURL, cfg, results)
	}()

	return results
}

func (s *Source) Enumerate(searchReqURL string, cfg *sources.Configuration, results chan sources.Result) {
	token := s.keysManager.GetCurrentKey()

	if token.RetryAfter > 0 {
		if len(s.keysManager.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = s.keysManager.GetCurrentKey()
		}
	}

	codeSearchResCFG := &hqgohttp.RequestConfiguration{
		Headers: []hqgohttp.Header{
			hqgohttp.NewSetHeader(hqgohttpheader.Accept.String(), "application/vnd.github.v3.text-match+json"),
			hqgohttp.NewSetHeader(hqgohttpheader.Authorization.String(), "token "+token.Value),
		},
	}

	codeSearchRes, err := hqgohttp.Get(searchReqURL, codeSearchResCFG)

	isForbidden := codeSearchRes != nil && codeSearchRes.StatusCode == hqgohttpstatus.Forbidden.Int()

	if err != nil && !isForbidden {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: s.Name(),
			Error:  fmt.Errorf("request failed: %w", err),
		}

		results <- result

		return
	}

	ratelimitRemaining := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.XRatelimitRemaining.String()))
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(codeSearchRes.Header.Get(hqgohttpheader.RetryAfter.String()))

		s.keysManager.SetCurrentKeyExceeded(retryAfterSeconds)

		s.Enumerate(searchReqURL, cfg, results)
	}

	var codeSearchResData codeSearchResponse

	if err = json.NewDecoder(codeSearchRes.Body).Decode(&codeSearchResData); err != nil {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: s.Name(),
			Error:  fmt.Errorf("failed to parse JSON response: %w", err),
		}

		results <- result

		codeSearchRes.Body.Close()

		return
	}

	codeSearchRes.Body.Close()

	for _, item := range codeSearchResData.Items {
		getRawContentReqURL := strings.ReplaceAll(item.HTMLURL, "https://github.com/", "https://raw.githubusercontent.com/")
		getRawContentReqURL = strings.ReplaceAll(getRawContentReqURL, "/blob/", "/")

		var getRawContentRes *http.Response

		getRawContentRes, err = hqgohttp.Get(getRawContentReqURL)
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
			getRawContentRes.Body.Close()

			continue
		}

		scanner := bufio.NewScanner(getRawContentRes.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			URLs := cfg.Extractor.FindAllString(line, -1)

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

			getRawContentRes.Body.Close()

			continue
		}

		getRawContentRes.Body.Close()

		for _, textMatch := range item.TextMatches {
			URLs := cfg.Extractor.FindAllString(textMatch.Fragment, -1)

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

	links := hqgohttpheaderutils.ParseLinkHeaderValue(codeSearchRes.Header.Get(hqgohttpheader.Link.String()))

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

			s.Enumerate(nextURL, cfg, results)
		}
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

func (m *KeysManager) GetCurrentKey() (key *ManagedKey) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.pool {
		key := &m.pool[i]

		if key.RetryAfter > 0 && time.Since(key.ExceededTime) > time.Duration(key.RetryAfter)*time.Second {
			key.ExceededTime = time.Time{}
			key.RetryAfter = 0
		}
	}

	if m.current >= len(m.pool) {
		m.current %= len(m.pool)
	}

	key = &m.pool[m.current]

	m.current++

	return
}

func (m *KeysManager) SetCurrentKeyExceeded(retryAfter int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current >= len(m.pool) {
		m.current %= len(m.pool)
	}

	key := &m.pool[m.current]

	if key.RetryAfter == 0 {
		key.ExceededTime = time.Now()
		key.RetryAfter = retryAfter
	}
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
