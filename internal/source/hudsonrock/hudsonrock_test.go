package hudsonrock_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/hudsonrock"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json/v2/osint-tools/urls-by-domain" || r.URL.Query().Get("domain") != "example.com" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"employees_urls":[{"url":"https://a.example.com/x"}],"clients_urls":[{"url":"https://b.example.com/y"}]}}`))
	}))
	defer server.Close()

	target, _ := url.Parse(server.URL)
	doer := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host

		return http.DefaultTransport.RoundTrip(clone)
	})}

	client, err := hudsonrock.New(doer)
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

	if len(values) != 2 || values[0] != "https://a.example.com/x" || values[1] != "https://b.example.com/y" {
		t.Fatalf("values = %v; want employee and client URLs", values)
	}
}

func TestSearchRejectsMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{"))
	}))
	defer server.Close()

	target, _ := url.Parse(server.URL)
	doer := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host

		return http.DefaultTransport.RoundTrip(clone)
	})}

	client, err := hudsonrock.New(doer)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com")
	if result.Err == nil {
		t.Fatal("Search accepted malformed JSON")
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := hudsonrock.New(nil); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := hudsonrock.New(&http.Client{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
