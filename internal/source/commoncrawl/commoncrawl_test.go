package commoncrawl_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hueristiq/xurlfind3r/internal/source/commoncrawl"
)

type rewriteTransport struct {
	target *url.URL
}

func (t *rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host

	response, err := http.DefaultTransport.RoundTrip(clone)
	if err != nil {
		return nil, fmt.Errorf("commoncrawl: test transport: %w", err)
	}

	return response, nil
}

func testClient(server *httptest.Server) *http.Client {
	target, _ := url.Parse(server.URL)

	return &http.Client{Transport: &rewriteTransport{target: target}}
}

func newProvider(t *testing.T, pages uint, bodies map[string]string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/collinfo.json":
			_, _ = fmt.Fprintf(writer, `[{"id":"CC-MAIN-%d-01","cdx-api":"https://index.test/cdx"}]`, time.Now().Year())
		case "/cdx":
			if r.URL.Query().Get("url") != "*.example.com" || r.URL.Query().Get("collapse") != "urlkey" {
				t.Errorf("query = %q", r.URL.RawQuery)
			}

			if r.URL.Query().Get("showNumPages") == "true" {
				_, _ = fmt.Fprintf(writer, `{"pages":%d}`, pages)

				return
			}

			requests.Add(1)

			_, _ = writer.Write([]byte(bodies[r.URL.Query().Get("page")]))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))

	return server, requests
}

func TestSearch(t *testing.T) {
	t.Parallel()

	server, _ := newProvider(t, 1, map[string]string{
		"0": "not-json\n" + `{"error":"bad domain"}` + "\n" + `{"url":"https://a.example.com/x"}` + "\n",
	})
	defer server.Close()

	client, err := commoncrawl.New(testClient(server))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	errorsSeen := 0

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			errorsSeen++

			continue
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "https://a.example.com/x" || errorsSeen != 2 {
		t.Fatalf("values = %v, errors = %d; want one value and two errors", values, errorsSeen)
	}
}

func TestSearchCustomRateLimitWait(t *testing.T) {
	t.Parallel()

	server, _ := newProvider(t, 2, map[string]string{
		"0": `{"url":"https://a.example.com/x"}` + "\n",
		"1": `{"url":"https://b.example.com/y"}` + "\n",
	})
	defer server.Close()

	var waits atomic.Int32

	client, err := commoncrawl.New(testClient(server), commoncrawl.WithRateLimitWait(func(context.Context) error {
		waits.Add(1)

		return nil
	}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 2 || waits.Load() != 2 {
		t.Fatalf("values = %v, waits = %d; want two pages", values, waits.Load())
	}
}

func TestSearchStopsAfterThreeEmptyPages(t *testing.T) {
	t.Parallel()

	server, requests := newProvider(t, 10, nil)
	defer server.Close()

	client, err := commoncrawl.New(testClient(server), commoncrawl.WithRateLimitWait(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(context.Background(), "example.com") {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}
	}

	if requests.Load() != 3 {
		t.Fatalf("page requests = %d; want 3", requests.Load())
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := commoncrawl.New(nil); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := commoncrawl.New(&http.Client{}, commoncrawl.WithRateLimitWait(nil)); err == nil {
		t.Fatal("New accepted a nil wait function")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := commoncrawl.New(&http.Client{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com") {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
