// Package testutil provides helpers for testing sources against
// httptest fixtures.
package testutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

// NewClient returns an HTTP client whose requests are all routed to
// server, so sources with hardcoded production URLs can be exercised
// against httptest fixtures.
//
// Parameters:
//   - t (*testing.T): the calling test.
//   - server (*httptest.Server): the fixture server.
//
// Returns:
//   - client (*hqgohttp.Client): the fixture-routed client.
func NewClient(t *testing.T, server *httptest.Server) (client *hqgohttp.Client) {
	t.Helper()

	httpClient := &http.Client{
		Transport: Transport{
			Target: server.Listener.Addr().String(),
		},
	}

	client, err := hqgohttp.NewClient(
		hqgohttp.WithHTTPClient(httpClient),
		hqgohttp.WithMaxRetries(0),
		hqgohttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create HTTP client: %v", err)
	}

	return client
}

// NewConfiguration returns a source configuration scoped to domain,
// with a validator mirroring the production scope rules.
//
// Parameters:
//   - t (*testing.T): the calling test.
//   - server (*httptest.Server): the fixture server.
//   - domain (string): the scope domain.
//
// Returns:
//   - cfg (*sources.Configuration): the fixture configuration.
func NewConfiguration(t *testing.T, server *httptest.Server, domain string) (cfg *sources.Configuration) {
	t.Helper()

	extractor, err := hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.PortOptionalPattern + `)`),
	).CompileRegex()
	if err != nil {
		t.Fatalf("failed to compile extractor: %v", err)
	}

	cfg = &sources.Configuration{
		Client:    NewClient(t, server),
		Extractor: extractor,
		Validator: func(target string) (URL string, valid bool) {
			switch {
			case strings.HasPrefix(target, "//"):
				URL = "https:" + target
			case strings.HasPrefix(target, "://"):
				URL = "https" + target
			case !strings.Contains(target, "//"):
				URL = "https://" + target
			default:
				URL = target
			}

			parsed, err := url.Parse(URL)
			if err != nil {
				return URL, false
			}

			hostname := strings.ToLower(parsed.Hostname())
			valid = hostname == domain || strings.HasSuffix(hostname, "."+domain)

			return URL, valid
		},
	}

	return cfg
}

// CollectURLs drains results, failing the test on any error result, and
// returns the emitted URLs in order.
//
// Parameters:
//   - t (*testing.T): the calling test.
//   - results (<-chan sources.Result): the result stream.
//
// Returns:
//   - URLs ([]string): the emitted URLs.
func CollectURLs(t *testing.T, results <-chan sources.Result) (URLs []string) {
	t.Helper()

	URLs = make([]string, 0)

	for result := range results {
		switch result.Type {
		case sources.ResultError:
			t.Errorf("unexpected error result: %v", result.Error)
		case sources.ResultURL:
			URLs = append(URLs, result.Value)
		}
	}

	return URLs
}

// Transport rewrites every request to its Target address, keeping the
// original path and query.
type Transport struct {
	Target string
}

func (t Transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	req = req.Clone(req.Context())

	req.URL.Scheme = "http"
	req.URL.Host = t.Target

	res, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("test transport round trip: %w", err)
	}

	return res, nil
}
