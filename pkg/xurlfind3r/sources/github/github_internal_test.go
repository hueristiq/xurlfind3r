package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	testKey1 = "key1"
	testKey2 = "key2"
)

func TestKeysManager(t *testing.T) {
	t.Parallel()

	t.Run("rotates round robin with indexes", func(t *testing.T) {
		t.Parallel()

		manager := NewKeyManager([]string{testKey1, testKey2})

		key, index := manager.GetCurrentKey()
		if key.Value != testKey1 || index != 0 {
			t.Fatalf("got (%q, %d), want (%q, %d)", key.Value, index, testKey1, 0)
		}

		key, index = manager.GetCurrentKey()
		if key.Value != testKey2 || index != 1 {
			t.Fatalf("got (%q, %d), want (%q, %d)", key.Value, index, testKey2, 1)
		}

		key, index = manager.GetCurrentKey()
		if key.Value != testKey1 || index != 0 {
			t.Fatalf("got (%q, %d), want (%q, %d)", key.Value, index, testKey1, 0)
		}
	})

	t.Run("marks the key that was used", func(t *testing.T) {
		t.Parallel()

		manager := NewKeyManager([]string{testKey1, testKey2})

		_, index := manager.GetCurrentKey()

		manager.SetKeyExceeded(index, 60)

		if manager.pool[index].RetryAfter != 60 {
			t.Fatalf("pool[%d].RetryAfter = %d, want 60", index, manager.pool[index].RetryAfter)
		}

		if manager.pool[1-index].RetryAfter != 0 {
			t.Fatalf("pool[%d].RetryAfter = %d, want 0", 1-index, manager.pool[1-index].RetryAfter)
		}

		key, _ := manager.GetCurrentKey()
		if key.Value == testKey1 {
			t.Fatalf("rate-limited key was handed out again")
		}
	})

	t.Run("reactivates an expired key", func(t *testing.T) {
		t.Parallel()

		manager := NewKeyManager([]string{testKey1, testKey2})

		_, index := manager.GetCurrentKey()

		manager.SetKeyExceeded(index, 60)

		manager.pool[index].ExceededTime = time.Now().Add(-2 * time.Minute)

		key, _ := manager.GetCurrentKey()
		if key.Value != testKey2 {
			t.Fatalf("got %q, want %q", key.Value, testKey2)
		}

		key, _ = manager.GetCurrentKey()
		if key.Value != testKey1 {
			t.Fatalf("got %q, want %q", key.Value, testKey1)
		}

		if key.RetryAfter != 0 {
			t.Fatalf("RetryAfter = %d, want 0", key.RetryAfter)
		}
	})

	t.Run("out of range index is a no-op", func(t *testing.T) {
		t.Parallel()

		manager := NewKeyManager([]string{testKey1})

		manager.SetKeyExceeded(99, 60)

		if manager.pool[0].RetryAfter != 0 {
			t.Fatalf("pool[0].RetryAfter = %d, want 0", manager.pool[0].RetryAfter)
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		t.Parallel()

		manager := NewKeyManager([]string{testKey1, testKey2, "key3"})

		wg := &sync.WaitGroup{}

		for i := range 10 {
			wg.Add(1)

			go func(i int) {
				defer wg.Done()

				_, index := manager.GetCurrentKey()

				if i%2 == 0 {
					manager.SetKeyExceeded(index, 1)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestSourceEnumerate(t *testing.T) {
	t.Parallel()

	t.Run("extracts URLs from search results and raw content", func(t *testing.T) {
		t.Parallel()

		queries := make(chan string, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/search/code":
				queries <- r.URL.Query().Get("q")

				w.Header().Set("Content-Type", "application/json")

				writeResponse(t, w, `{"total_count":1,"items":[{"name":"README.md","html_url":"https://github.com/hueristiq/xurlfind3r/blob/main/README.md","text_matches":[{"fragment":"see https://example.com/x for details"}]}]}`)
			case "/hueristiq/xurlfind3r/main/README.md":
				w.Header().Set("Content-Type", "text/plain")

				writeResponse(t, w, "https://example.com/a\nhttps://sub.example.com/b\nhttps://example.com.evil.com/phantom\n")
			default:
				t.Errorf("unexpected request path: %s", r.URL.Path)

				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := newTestSource()
		cfg := testConfiguration(t, server)

		results := source.EnumerateForTest(context.Background(), cfg)

		want := []string{
			"https://example.com/x",
			"https://example.com/a",
			"https://sub.example.com/b",
		}

		got := collectURLs(t, results)

		slices.Sort(got)
		slices.Sort(want)

		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}

		if query := <-queries; query != "example.com" {
			t.Errorf("q parameter = %q, want %q", query, "example.com")
		}
	})

	t.Run("rate limit switches key and does not decode the 403 body", func(t *testing.T) {
		t.Parallel()

		authHeaders := make(chan string, 2)
		requestCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/search/code":
				authHeaders <- r.Header.Get("Authorization")

				requestCount++

				if requestCount == 1 {
					w.Header().Set("X-Ratelimit-Remaining", "0")
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusForbidden)

					writeResponse(t, w, "rate limited")

					return
				}

				w.Header().Set("Content-Type", "application/json")

				writeResponse(t, w, `{"total_count":1,"items":[{"name":"f.txt","html_url":"https://github.com/hueristiq/xurlfind3r/blob/main/f.txt","text_matches":[{"fragment":"https://example.com/x"}]}]}`)
			case "/hueristiq/xurlfind3r/main/f.txt":
				w.Header().Set("Content-Type", "text/plain")

				writeResponse(t, w, "")
			default:
				t.Errorf("unexpected request path: %s", r.URL.Path)

				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := newTestSource()
		cfg := testConfiguration(t, server)

		results := source.EnumerateForTest(context.Background(), cfg)

		got := collectURLs(t, results)

		if len(got) != 1 || got[0] != "https://example.com/x" {
			t.Fatalf("got %v, want [https://example.com/x]", got)
		}

		first := <-authHeaders
		second := <-authHeaders

		if first != "token "+testKey1 {
			t.Errorf("first request auth = %q, want %q", first, "token "+testKey1)
		}

		if second != "token "+testKey2 {
			t.Errorf("second request auth = %q, want %q", second, "token "+testKey2)
		}
	})

	t.Run("single key sleep is context aware", func(t *testing.T) {
		t.Parallel()

		source := &Source{
			keys: sources.Keys{testKey1},
		}

		source.keysManager = NewKeyManager(source.keys)
		source.keysManager.SetKeyExceeded(0, 2)

		cfg := &sources.Configuration{
			Client: noopTestClient(t),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		results := make(chan sources.Result, 4)

		start := time.Now()

		source.Enumerate(ctx, "https://api.github.com/search/code", cfg, results)

		elapsed := time.Since(start)

		if elapsed > time.Second {
			t.Fatalf("Enumerate took %v; context cancellation was not honored", elapsed)
		}

		if len(results) != 0 {
			t.Fatalf("got %d results, want 0", len(results))
		}
	})
}

// EnumerateForTest runs Enumerate against the test client and returns
// its result stream.
func (s *Source) EnumerateForTest(ctx context.Context, cfg *sources.Configuration) (results chan sources.Result) {
	results = make(chan sources.Result, 16)

	parameters := url.Values{}
	parameters.Set("per_page", "100")
	parameters.Set("q", "example.com")
	parameters.Set("sort", "created")
	parameters.Set("order", "asc")

	go func() {
		defer close(results)

		s.Enumerate(ctx, "https://api.github.com/search/code?"+parameters.Encode(), cfg, results)
	}()

	return results
}

func collectURLs(t *testing.T, results chan sources.Result) (URLs []string) {
	t.Helper()

	URLs = make([]string, 0)

	for result := range results {
		switch result.Type {
		case sources.ResultError:
			t.Errorf("unexpected error result: %v", result.Error)
		case sources.ResultURL:
			URLs = append(URLs, result.Value)
		}
	}

	return URLs
}

func newTestSource() (source *Source) {
	source = &Source{
		keys: sources.Keys{testKey1, testKey2},
	}

	source.keysManager = NewKeyManager(source.keys)

	return source
}

func testConfiguration(t *testing.T, server *httptest.Server) (cfg *sources.Configuration) {
	t.Helper()

	extractor, err := hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*example\.com` + hqgourlextractor.PortOptionalPattern + `)`),
	).CompileRegex()
	if err != nil {
		t.Fatalf("failed to compile extractor: %v", err)
	}

	cfg = &sources.Configuration{
		Client:    testClient(t, server),
		Extractor: extractor,
		Validator: func(target string) (URL string, valid bool) {
			switch {
			case strings.HasPrefix(target, "//"):
				URL = "https:" + target
			case strings.HasPrefix(target, "://"):
				URL = "https" + target
			case !strings.Contains(target, "//"):
				URL = "https://" + target
			default:
				URL = target
			}

			parsed, err := url.Parse(URL)
			if err != nil {
				return URL, false
			}

			hostname := strings.ToLower(parsed.Hostname())
			valid = hostname == "example.com" || strings.HasSuffix(hostname, ".example.com")

			return URL, valid
		},
	}

	return cfg
}

func noopTestClient(t *testing.T) (client *hqgohttp.Client) {
	t.Helper()

	client, err := hqgohttp.NewClient(
		hqgohttp.WithHTTPClient(&http.Client{Transport: http.DefaultTransport}),
		hqgohttp.WithMaxRetries(0),
		hqgohttp.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client
}

func testClient(t *testing.T, server *httptest.Server) (client *hqgohttp.Client) {
	t.Helper()

	httpClient := &http.Client{
		Transport: testTransport{
			target: server.Listener.Addr().String(),
		},
	}

	client, err := hqgohttp.NewClient(
		hqgohttp.WithHTTPClient(httpClient),
		hqgohttp.WithMaxRetries(0),
		hqgohttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return client
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}

// testTransport rewrites every request to the test server's address so
// hardcoded production URLs can be exercised against httptest fixtures.
type testTransport struct {
	target string
}

func (t testTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	req = req.Clone(req.Context())

	req.URL.Scheme = "http"
	req.URL.Host = t.target

	res, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("test transport round trip: %w", err)
	}

	return res, nil
}
