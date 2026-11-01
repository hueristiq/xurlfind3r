package virustotal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/virustotal"
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

func TestSubdomains(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/domains/example.com/subdomains" || r.Header.Get("X-Apikey") != "secret" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("cursor") == "" {
			_, _ = writer.Write([]byte(`{"data":[{"id":"a.example.com"}],"meta":{"cursor":"next"}}`))

			return
		}

		_, _ = writer.Write([]byte(`{"data":[{"id":"b.example.com"}],"meta":{"cursor":""}}`))
	}))
	defer server.Close()

	var waits atomic.Int32

	client, err := virustotal.New(testClient(server), "secret", virustotal.WithRateLimitWait(func(context.Context) error {
		waits.Add(1)

		return nil
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Subdomains(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Subdomains: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 2 || waits.Load() != 2 {
		t.Fatalf("values = %v, waits = %d; want two pages", values, waits.Load())
	}
}

func TestSubdomainsWarnsOnRepeatedCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"data":[{"id":"a.example.com"}],"meta":{"cursor":"same"}}`))
	}))
	defer server.Close()

	client, err := virustotal.New(testClient(server), "secret", virustotal.WithRateLimitWait(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	values := 0
	warnings := 0

	for result := range client.Subdomains(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Subdomains: %v", result.Err)
		}

		if result.Warning != "" {
			warnings++
		}

		if result.Value != "" {
			values++
		}
	}

	if values != 2 || warnings != 1 {
		t.Fatalf("values = %d, warnings = %d; want 2 and 1", values, warnings)
	}
}

func TestDomainReport(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vtapi/v2/domain/report" || r.URL.Query().Get("apikey") != "secret" || r.URL.Query().Get("domain") != "example.com" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"detected_urls":[{"url":"https://a.example.com/x"}],"subdomains":["b.example.com"],"undetected_urls":[["https://c.example.com/z",1]]}`))
	}))
	defer server.Close()

	client, err := virustotal.New(testClient(server), "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.DomainReport(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("DomainReport: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 3 {
		t.Fatalf("values = %v; want all three URL classes", values)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := virustotal.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := virustotal.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}

	if _, err := virustotal.New(&http.Client{}, "secret", virustotal.WithRateLimitWait(nil)); err == nil {
		t.Fatal("New accepted a nil wait function")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := virustotal.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Subdomains(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
