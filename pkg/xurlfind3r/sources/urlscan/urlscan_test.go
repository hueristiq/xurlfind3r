package urlscan_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
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

func TestURLScanRepeatedCursor(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"results":[{"page":{"url":"https://example.com/a"},"sort":["cursor"]}],"has_more":true}`))
	}))
	t.Cleanup(server.Close)

	source := urlscan.New()
	source.UseKeys("key")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	var urls, warnings int

	for _, result := range results {
		switch result.Type {
		case sources.ResultURL:
			urls++
		case sources.ResultWarning:
			warnings++

			if !strings.Contains(result.Value, "cursor did not advance") {
				t.Fatalf("warning = %q, want cursor warning", result.Value)
			}
		case sources.ResultError:
			t.Fatalf("Run() error result: %v", result.Error)
		}
	}

	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}

	if urls != 2 || warnings != 1 {
		t.Fatalf("results = %d URLs, %d warnings; want 2 and 1", urls, warnings)
	}
}

func TestURLScanPageCap(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		page := requests.Add(1)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{"results":[{"page":{"url":"https://example.com/%d"},"sort":["%d"]}],"has_more":true}`, page, page)
	}))
	t.Cleanup(server.Close)

	source := urlscan.New()
	source.UseKeys("key")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	var capped bool

	for _, result := range results {
		if result.Type == sources.ResultWarning && strings.Contains(result.Value, "capped") {
			capped = true
		}

		if result.Type == sources.ResultError {
			t.Fatalf("Run() error result: %v", result.Error)
		}
	}

	if got := requests.Load(); got != sources.MaxPages {
		t.Fatalf("requests = %d, want %d", got, sources.MaxPages)
	}

	if !capped {
		t.Fatal("Run() did not emit a page-cap warning")
	}
}

func TestURLScanStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("bad request"))
	}))
	t.Cleanup(server.Close)

	source := urlscan.New()
	source.UseKeys("key")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	if len(results) != 1 || results[0].Type != sources.ResultError || !strings.Contains(results[0].Error.Error(), "400") {
		t.Fatalf("Run() results = %v, want one 400 error", results)
	}
}
