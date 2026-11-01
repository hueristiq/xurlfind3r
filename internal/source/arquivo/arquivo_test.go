package arquivo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/arquivo"
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
		if r.URL.Path != "/wayback/cdx" || r.URL.Query().Get("url") != "*.example.com/*" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		_, _ = writer.Write([]byte("\nnot-json\n" + `{"url":"https://a.example.com/x"}` + "\n" + `{"url":"https://b.example.com/y"}` + "\n"))
	}))
	defer server.Close()

	client, err := arquivo.New(testClient(server))
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

func TestSearchRejectsStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := arquivo.New(testClient(server))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com")
	if result.Err == nil {
		t.Fatal("Search accepted a 503 response")
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := arquivo.New(nil); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := arquivo.New(&http.Client{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
