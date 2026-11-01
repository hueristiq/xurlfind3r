package leakradar_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/leakradar"
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
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing bearer credential")
		}

		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = writer.Write([]byte(`{"items":[{"url":"https://a.example.com/x"}],"total":2,"page":1,"page_size":1}`))

			return
		}

		_, _ = writer.Write([]byte(`{"items":[{"url":"https://b.example.com/y"}],"total":2,"page":2,"page_size":1}`))
	}))
	defer server.Close()

	client, err := leakradar.New(testClient(server), "secret")
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

	if len(values) != 2 {
		t.Fatalf("values = %v; want two URLs", values)
	}
}

func TestSearchWarnsOnRepeatedPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"items":[{"url":"https://a.example.com/x"}],"total":2,"page":1,"page_size":1}`))
	}))
	defer server.Close()

	client, err := leakradar.New(testClient(server), "secret")
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

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := leakradar.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := leakradar.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := leakradar.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
