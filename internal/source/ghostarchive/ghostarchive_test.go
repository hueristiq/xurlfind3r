package ghostarchive_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/ghostarchive"
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
		if r.URL.Query().Get("term") != ".example.com" {
			t.Errorf("term = %q; want .example.com", r.URL.Query().Get("term"))
		}

		_, _ = writer.Write([]byte(`<a href="/archive/1">https://a.example.com/x</a>`))
	}))
	defer server.Close()

	client, err := ghostarchive.New(testClient(server))
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

	if len(values) != 1 || values[0] != "https://a.example.com/x" {
		t.Fatalf("values = %v; want one archived URL", values)
	}
}

func TestSearchPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "0" {
			_, _ = writer.Write([]byte(`<a href="/archive/1">https://a.example.com/x</a>Next Page`))

			return
		}

		_, _ = writer.Write([]byte(`<a href="/archive/2">https://b.example.com/y</a>`))
	}))
	defer server.Close()

	var waits atomic.Int32

	client, err := ghostarchive.New(testClient(server), ghostarchive.WithRateLimitWait(func(context.Context) error {
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

	if len(values) != 2 || waits.Load() != 2 {
		t.Fatalf("values = %v, waits = %d; want two pages", values, waits.Load())
	}
}

func TestSearchStopsRepeatedPage(t *testing.T) {
	t.Parallel()

	body := `<a href="/archive/1">https://a.example.com/x</a>Next Page`

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(body))
	}))
	defer server.Close()

	client, err := ghostarchive.New(testClient(server), ghostarchive.WithRateLimitWait(func(context.Context) error { return nil }))
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

	if len(values) != 1 {
		t.Fatalf("values = %v; repeated page must stop pagination", values)
	}
}

func TestSearchReportsMaintenance(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("The site is under maintenance"))
	}))
	defer server.Close()

	client, err := ghostarchive.New(testClient(server))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com")
	if result.Err == nil {
		t.Fatal("Search accepted a maintenance response")
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := ghostarchive.New(nil); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := ghostarchive.New(&http.Client{}, ghostarchive.WithRateLimitWait(nil)); err == nil {
		t.Fatal("New accepted a nil wait function")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := ghostarchive.New(&http.Client{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
