package github_test

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
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
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

func TestGitHubRateLimitRecovery(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("sort"); got != "indexed" {
			t.Errorf("sort query = %q, want indexed", got)
		}

		if requests.Add(1) == 1 {
			writer.Header().Set("X-Ratelimit-Remaining", "0")
			writer.Header().Set("Retry-After", "1")
			writer.WriteHeader(http.StatusForbidden)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"total_count":0,"items":[]}`))
	}))
	t.Cleanup(server.Close)

	source := github.New()
	source.UseKeys("token")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	for _, result := range results {
		if result.Type == sources.ResultError {
			t.Fatalf("Run() error result: %v", result.Error)
		}
	}

	if got := requests.Load(); got < 2 {
		t.Fatalf("requests = %d, want at least 2", got)
	}
}

func TestGitHubStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("bad credentials"))
	}))
	t.Cleanup(server.Close)

	source := github.New()
	source.UseKeys("token")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	var found bool

	for _, result := range results {
		if result.Type == sources.ResultError && strings.Contains(result.Error.Error(), "401") {
			found = true
		}
	}

	if !found {
		t.Fatalf("Run() results = %v, want a 401 error", results)
	}
}

func TestGitHubWithoutKeys(t *testing.T) {
	t.Parallel()

	source := github.New()

	cfg := &sources.Configuration{Validator: acceptAll}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	if len(results) != 0 {
		t.Fatalf("Run() without keys = %v, want no results", results)
	}
}
