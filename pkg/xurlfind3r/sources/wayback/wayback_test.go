package wayback_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

const exampleDomain = "example.com"

type rewriteTransport struct {
	base   *http.Transport
	target *url.URL
}

func (transport *rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = transport.target.Scheme
	clone.URL.Host = transport.target.Host
	clone.Host = transport.target.Host

	response, err := transport.base.RoundTrip(clone)
	if err != nil {
		return nil, fmt.Errorf("round trip: %w", err)
	}

	return response, nil
}

func newClient(t *testing.T, serverURL string) (client *hqgohttp.Client) {
	t.Helper()

	target, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	base := &http.Transport{Proxy: nil}

	t.Cleanup(base.CloseIdleConnections)

	httpClient := &http.Client{
		Transport: &rewriteTransport{
			base:   base,
			target: target,
		},
	}

	client, err = hqgohttp.NewClient(hqgohttp.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	return
}

func acceptAll(target string) (string, bool) {
	return target, true
}

func collectResults(t *testing.T, results <-chan sources.Result) (collected []sources.Result) {
	t.Helper()

	for result := range results {
		collected = append(collected, result)
	}

	return
}

//nolint:paralleltest // the package-level rate limiter is shared across tests.
func TestWaybackRepeatedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[["timestamp","original"],["20200101000000","https://example.com/a"]]`))
	}))
	t.Cleanup(server.Close)

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, wayback.New().Run(t.Context(), cfg, exampleDomain))

	var urls, warnings int

	for _, result := range results {
		switch result.Type {
		case sources.ResultURL:
			urls++
		case sources.ResultWarning:
			warnings++

			if !strings.Contains(result.Value, "repeated") {
				t.Fatalf("warning = %q, want repeated-page warning", result.Value)
			}
		case sources.ResultError:
			t.Fatalf("Run() error result: %v", result.Error)
		}
	}

	if urls != 1 || warnings != 1 {
		t.Fatalf("results = %d URLs, %d warnings; want 1 and 1", urls, warnings)
	}
}

//nolint:paralleltest // the package-level rate limiter is shared across tests.
func TestWaybackStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("bad request"))
	}))
	t.Cleanup(server.Close)

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, wayback.New().Run(t.Context(), cfg, exampleDomain))

	if len(results) != 1 || results[0].Type != sources.ResultError || !strings.Contains(results[0].Error.Error(), "400") {
		t.Fatalf("Run() results = %v, want one 400 error", results)
	}
}
