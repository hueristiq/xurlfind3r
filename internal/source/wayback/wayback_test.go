package wayback_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hueristiq/xurlfind3r/internal/source/wayback"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testClient(server *httptest.Server) *http.Client {
	target, _ := url.Parse(server.URL)

	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host

		return http.DefaultTransport.RoundTrip(clone)
	})}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdx/search/cdx" || r.URL.Query().Get("url") != "*.example.com/*" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "0" {
			_, _ = writer.Write([]byte(`[["timestamp","original"],["1","https://a.example.com/x"]]`))

			return
		}

		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()

	var waits atomic.Int32

	client, err := wayback.New(testClient(server), wayback.WithRateLimitWait(func(context.Context) error {
		waits.Add(1)

		return nil
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "https://a.example.com/x" || waits.Load() != 2 {
		t.Fatalf("values = %v, waits = %d; want one value after two pages", values, waits.Load())
	}
}

func TestSearchWarnsOnRepeatedPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[["timestamp","original"],["1","https://a.example.com/x"]]`))
	}))
	defer server.Close()

	client, err := wayback.New(testClient(server), wayback.WithRateLimitWait(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	values := 0
	warnings := 0

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		if result.Warning != "" {
			warnings++
		}

		if result.Value != "" {
			values++
		}
	}

	if values != 1 || warnings != 1 {
		t.Fatalf("values = %d, warnings = %d; want 1 and 1", values, warnings)
	}
}

func TestSearchRetriesRateLimit(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writer.WriteHeader(http.StatusTooManyRequests)

			return
		}

		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()

	client, err := wayback.New(testClient(server),
		wayback.WithRateLimitWait(func(context.Context) error { return nil }),
		wayback.WithRetryPolicy(time.Millisecond, 1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}
	}

	if requests.Load() != 2 {
		t.Fatalf("requests = %d; want one retry", requests.Load())
	}
}

func TestRetryPolicyDoesNotDuplicate429Retry(t *testing.T) {
	t.Parallel()

	retry, err := wayback.RetryPolicy(context.Background(), nil, &http.Response{StatusCode: http.StatusTooManyRequests}, nil, 1)
	if err != nil || retry {
		t.Fatalf("RetryPolicy = %t, %v; want false, nil", retry, err)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := wayback.New(nil); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := wayback.New(&http.Client{}, wayback.WithRateLimitWait(nil)); err == nil {
		t.Fatal("New accepted a nil wait function")
	}

	if _, err := wayback.New(&http.Client{}, wayback.WithRetryPolicy(0, -1)); err == nil {
		t.Fatal("New accepted an invalid retry policy")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := wayback.New(&http.Client{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
