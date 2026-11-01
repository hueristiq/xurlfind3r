package otx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/otx"
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

func TestPassiveDNS(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/indicators/domain/example.com/passive_dns" || r.Header.Get("X-Otx-Api-Key") != "secret" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"passive_dns":[{"hostname":"a.example.com"}]}`))
	}))
	defer server.Close()

	client, err := otx.New(testClient(server), "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.PassiveDNS(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("PassiveDNS: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "a.example.com" {
		t.Fatalf("values = %v; want a.example.com", values)
	}
}

func TestURLs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = writer.Write([]byte(`{"url_list":[{"url":"https://a.example.com/x"}],"actual_size":1,"has_next":true}`))

			return
		}

		_, _ = writer.Write([]byte(`{"url_list":[{"url":"https://b.example.com/y"}],"actual_size":1,"has_next":false}`))
	}))
	defer server.Close()

	client, err := otx.New(testClient(server), "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.URLs(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("URLs: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 2 {
		t.Fatalf("values = %v; want two URLs", values)
	}
}

func TestURLsWarnsOnRepeatedPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"url_list":[{"url":"https://a.example.com/x"}],"actual_size":1,"has_next":true}`))
	}))
	defer server.Close()

	client, err := otx.New(testClient(server), "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	values := 0
	warnings := 0

	for result := range client.URLs(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("URLs: %v", result.Err)
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

	if _, err := otx.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := otx.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := otx.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.URLs(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
