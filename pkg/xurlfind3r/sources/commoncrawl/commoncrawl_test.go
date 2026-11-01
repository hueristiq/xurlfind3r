package commoncrawl_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestRunMapsURLs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/collinfo.json":
			_, _ = fmt.Fprintf(writer, `[{"id":"CC-MAIN-%d-01","cdx-api":"https://index.test/cdx"}]`, time.Now().Year())
		case "/cdx":
			if r.URL.Query().Get("showNumPages") == "true" {
				_, _ = writer.Write([]byte(`{"pages":1}`))

				return
			}

			_, _ = writer.Write([]byte(`{"url":"https://api.example.com/x"}` + "\n"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
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

	source := commoncrawl.New()

	var values []string

	for result := range source.Run(context.Background(), cfg, "example.com") {
		if result.Error != nil {
			t.Fatalf("Run: %v", result.Error)
		}

		if result.Type != sources.ResultURL {
			t.Fatalf("result type = %v; want URL", result.Type)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "https://api.example.com/x" {
		t.Fatalf("values = %v; want the provider URL", values)
	}
}
