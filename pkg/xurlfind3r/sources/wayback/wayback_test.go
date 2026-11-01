package wayback_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRunMapsURLs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cdx/search/cdx" || r.URL.Query().Get("url") != "*.example.com/*" {
			t.Errorf("unexpected request: %s", r.URL.String())
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[["timestamp","original"],["1","https://api.example.com/x"]]`))
	}))
	defer server.Close()

	target, _ := url.Parse(server.URL)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host

		return http.DefaultTransport.RoundTrip(clone)
	})

	client, err := hqgohttp.New(hqgohttp.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatalf("New HTTP client: %v", err)
	}

	cfg := &sources.Configuration{
		Client:    client,
		Validator: func(value string) (string, bool) { return value, true },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	source := wayback.New()

	var values []string

	for result := range source.Run(ctx, cfg, "example.com") {
		if result.Error != nil {
			t.Fatalf("Run: %v", result.Error)
		}

		if result.Type != sources.ResultURL {
			t.Fatalf("result type = %v; want URL", result.Type)
		}

		values = append(values, result.Value)

		cancel()
	}

	if len(values) != 1 || values[0] != "https://api.example.com/x" {
		t.Fatalf("values = %v; want the provider URL", values)
	}
}
