package urlscan_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/urlscan"
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

	for _, test := range []struct {
		name   string
		target urlscan.Target
		want   string
	}{
		{name: "domains", target: urlscan.Domains, want: "a.example.com"},
		{name: "URLs", target: urlscan.URLs, want: "https://a.example.com/x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Api-Key") != "secret" || r.Header.Get("Accept") != "application/json" {
					t.Error("missing urlscan headers")
				}

				writer.Header().Set("Content-Type", "application/json")

				if r.URL.Query().Get("search_after") == "" {
					_, _ = writer.Write([]byte(`{"results":[{"page":{"domain":"a.example.com","url":"https://a.example.com/x"},"sort":[123,"abc"]}],"has_more":true}`))

					return
				}

				if r.URL.Query().Get("search_after") != "123,abc" {
					t.Errorf("search_after = %q", r.URL.Query().Get("search_after"))
				}

				_, _ = writer.Write([]byte(`{"results":[{"page":{"domain":"b.example.com","url":"https://b.example.com/y"},"sort":[456]}],"has_more":false}`))
			}))
			defer server.Close()

			client, err := urlscan.New(testClient(server), "secret")
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			var values []string

			for result := range client.Search(context.Background(), "example.com", test.target) {
				if result.Err != nil {
					t.Fatalf("Search: %v", result.Err)
				}

				values = append(values, result.Value)
			}

			if len(values) != 2 || values[0] != test.want {
				t.Fatalf("values = %v; want first value %q", values, test.want)
			}
		})
	}
}

func TestSearchWarnsOnRepeatedCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"results":[{"page":{"domain":"a.example.com","url":"https://a.example.com/x"},"sort":[1]}],"has_more":true}`))
	}))
	defer server.Close()

	client, err := urlscan.New(testClient(server), "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	values := 0
	warnings := 0

	for result := range client.Search(context.Background(), "example.com", urlscan.Domains) {
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

	if values != 2 || warnings != 1 {
		t.Fatalf("values = %d, warnings = %d; want 2 and 1", values, warnings)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := urlscan.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := urlscan.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	client, err := urlscan.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com", 0)
	if result.Err == nil {
		t.Fatal("Search accepted an invalid target")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := urlscan.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com", urlscan.Domains) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
