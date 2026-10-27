package gitlab_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/gitlab"
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

	client = hqgohttp.New(hqgohttp.WithHTTPClient(httpClient))

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

func TestGitLabRepeatedPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-Next-Page", request.URL.Query().Get("page"))
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	source := gitlab.New()
	source.UseKeys("key")

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: acceptAll,
	}

	results := collectResults(t, source.Run(t.Context(), cfg, exampleDomain))

	if len(results) != 1 || results[0].Type != sources.ResultWarning || !strings.Contains(results[0].Value, "next page did not advance") {
		t.Fatalf("Run() results = %v, want one repeated-page warning", results)
	}
}

func TestGitLabStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("bad request"))
	}))
	t.Cleanup(server.Close)

	source := gitlab.New()
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
