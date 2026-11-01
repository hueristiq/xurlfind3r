package bevigil_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/bevigil"
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
		name     string
		target   bevigil.Target
		path     string
		response string
		want     string
	}{
		{name: "domains", target: bevigil.Domains, path: "/api/example.com/subdomains/", response: `{"subdomains":["a.example.com"]}`, want: "a.example.com"},
		{name: "URLs", target: bevigil.URLs, path: "/api/example.com/urls/", response: `{"urls":["https://a.example.com/x"]}`, want: "https://a.example.com/x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path {
					t.Errorf("path = %q; want %q", r.URL.Path, test.path)
				}

				if r.Header.Get("X-Access-Token") != "secret" {
					t.Error("missing API key header")
				}

				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()

			client, err := bevigil.New(testClient(server), "secret")
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

			if len(values) != 1 || values[0] != test.want {
				t.Fatalf("values = %v; want %q", values, test.want)
			}
		})
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := bevigil.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := bevigil.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	client, err := bevigil.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, ok := <-client.Search(context.Background(), "example.com", 0)
	if !ok || result.Err == nil {
		t.Fatalf("invalid target result = %+v, %t; want an error", result, ok)
	}
}

func TestSearchRedactsAPIKey(t *testing.T) {
	t.Parallel()

	original := errors.New("request failed with secret")
	doer := hqgosource.DoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, original
	})

	client, err := bevigil.New(doer, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com", bevigil.Domains)
	if result.Err == nil || strings.Contains(result.Err.Error(), "secret") {
		t.Fatalf("error = %v; want redacted API key", result.Err)
	}

	if !errors.Is(result.Err, original) {
		t.Fatal("redacted error does not unwrap to the transport error")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := bevigil.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com", bevigil.Domains) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
