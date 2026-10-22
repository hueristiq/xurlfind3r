package commoncrawl_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
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

func rejectSubdomains(target string) (string, bool) {
	parsed, err := url.Parse(target)
	if err != nil {
		return target, false
	}

	if strings.HasPrefix(parsed.Hostname(), "sub.") {
		return target, false
	}

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
func TestCommonCrawlScannedPageGuard(t *testing.T) {
	var server *httptest.Server

	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/collinfo.json":
			payload := fmt.Sprintf(`[{"id":"CC-MAIN-%d-01","cdx-api":"%s/cc-index"}]`, time.Now().Year(), server.URL)
			_, _ = writer.Write([]byte(payload))
		case request.URL.Query().Get("showNumPages") == "true":
			_, _ = writer.Write([]byte(`{"pages":4}`))
		default:
			page, _ := strconv.Atoi(request.URL.Query().Get("page"))

			switch page {
			case 0, 1, 2:
				_, _ = writer.Write([]byte(`{"url":"https://sub.example.com/rejected"}`))
			default:
				_, _ = writer.Write([]byte(`{"url":"https://www.example.com/found"}`))
			}
		}
	}))
	t.Cleanup(server.Close)

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: rejectSubdomains,
	}

	results := collectResults(t, commoncrawl.New().Run(t.Context(), cfg, exampleDomain))

	var found bool

	for _, result := range results {
		switch result.Type {
		case sources.ResultURL:
			if result.Value == "https://www.example.com/found" {
				found = true
			}
		case sources.ResultWarning:
		case sources.ResultError:
			t.Fatalf("Run() error result: %v", result.Error)
		}
	}

	if !found {
		t.Fatalf("Run() results = %v, want https://www.example.com/found", results)
	}
}

//nolint:paralleltest // the package-level rate limiter is shared across tests.
func TestCommonCrawlStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("bad request"))
	}))
	t.Cleanup(server.Close)

	cfg := &sources.Configuration{
		Client:    newClient(t, server.URL),
		Validator: rejectSubdomains,
	}

	results := collectResults(t, commoncrawl.New().Run(t.Context(), cfg, exampleDomain))

	if len(results) != 1 || results[0].Type != sources.ResultError || !strings.Contains(results[0].Error.Error(), "400") {
		t.Fatalf("Run() results = %v, want one 400 error", results)
	}
}
