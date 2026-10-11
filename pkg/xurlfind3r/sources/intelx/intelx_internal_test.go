package intelx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/testutil"
)

func TestSourceRun(t *testing.T) {
	t.Parallel()

	newSource := func(keys ...string) (source *Source) {
		source = &Source{
			keys:            keys,
			pollInterval:    time.Millisecond,
			pollAttemptsMax: 5,
		}

		return source
	}

	t.Run("rejects a malformed key", func(t *testing.T) {
		t.Parallel()

		source := newSource("not-a-valid-key")

		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()

		cfg := &sources.Configuration{
			Client: testutil.NewClient(t, server),
		}

		results := source.Run(context.Background(), cfg, "example.com")

		got := collectErrors(t, results)

		if len(got) != 1 {
			t.Fatalf("got %d error results, want 1", len(got))
		}
	})

	t.Run("rejects an empty key half", func(t *testing.T) {
		t.Parallel()

		source := newSource("2.intelx.io:")

		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()

		cfg := &sources.Configuration{
			Client: testutil.NewClient(t, server),
		}

		results := source.Run(context.Background(), cfg, "example.com")

		got := collectErrors(t, results)

		if len(got) != 1 {
			t.Fatalf("got %d error results, want 1", len(got))
		}
	})

	t.Run("polls until results are ready", func(t *testing.T) {
		t.Parallel()

		requestBodies := make(chan []byte, 1)
		pollCount := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/phonebook/search":
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}

				requestBodies <- body

				writeResponse(t, w, `{"id":"search-id","status":0}`)
			case "/phonebook/search/result":
				pollCount++

				if pollCount < 3 {
					writeResponse(t, w, `{"selectors":[],"status":3}`)
				} else {
					writeResponse(t, w, `{"selectors":[{"selectorvalue":"https://example.com/x"},{"selectorvalue":"https://example.com.evil.com/phantom"}],"status":1}`)
				}
			default:
				t.Errorf("unexpected request path: %s", r.URL.Path)

				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := newSource("2.intelx.io:test-key")

		cfg := testutil.NewConfiguration(t, server, "example.com")

		results := source.Run(context.Background(), cfg, "example.com")

		got := testutil.CollectURLs(t, results)

		if len(got) != 1 || got[0] != "https://example.com/x" {
			t.Fatalf("got %v, want [https://example.com/x]", got)
		}

		if pollCount != 3 {
			t.Fatalf("server saw %d result polls, want 3", pollCount)
		}

		body := <-requestBodies

		var searchRequest searchRequestBody

		if err := json.Unmarshal(body, &searchRequest); err != nil {
			t.Fatalf("failed to unmarshal search request: %v", err)
		}

		if searchRequest.Timeout != defaultSearchTimeout {
			t.Errorf("search timeout = %d, want %d", searchRequest.Timeout, defaultSearchTimeout)
		}
	})

	t.Run("gives up after the poll cap", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/phonebook/search":
				writeResponse(t, w, `{"id":"search-id","status":0}`)
			case "/phonebook/search/result":
				writeResponse(t, w, `{"selectors":[],"status":0}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := newSource("2.intelx.io:test-key")

		cfg := testutil.NewConfiguration(t, server, "example.com")

		results := source.Run(context.Background(), cfg, "example.com")

		got := collectErrors(t, results)

		if len(got) != 1 {
			t.Fatalf("got %d error results, want 1", len(got))
		}
	})

	t.Run("redacts the key from request errors", func(t *testing.T) {
		t.Parallel()

		source := newSource("2.intelx.io:secret-key")

		httpClient := &http.Client{
			Transport: failingTransport{},
		}

		client, err := hqgohttp.NewClient(
			hqgohttp.WithHTTPClient(httpClient),
			hqgohttp.WithMaxRetries(0),
			hqgohttp.WithTimeout(2*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		cfg := &sources.Configuration{
			Client: client,
		}

		results := source.Run(context.Background(), cfg, "example.com")

		got := collectErrors(t, results)

		if len(got) != 1 {
			t.Fatalf("got %d error results, want 1", len(got))
		}

		if strings.Contains(got[0].Error(), "secret-key") {
			t.Errorf("error leaks the key: %v", got[0])
		}

		if !strings.Contains(got[0].Error(), "[REDACTED]") {
			t.Errorf("error does not redact the key: %v", got[0])
		}
	})
}

// failingTransport fails every request with the request URL embedded in
// the error, mimicking a transport failure a source must redact.
type failingTransport struct{}

func (t failingTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	return nil, fmt.Errorf("dial tcp: %s", req.URL.String())
}

func collectErrors(t *testing.T, results <-chan sources.Result) (errs []error) {
	t.Helper()

	errs = make([]error, 0)

	for result := range results {
		if result.Type != sources.ResultError {
			t.Errorf("unexpected result type %v", result.Type)

			continue
		}

		errs = append(errs, result.Error)
	}

	return errs
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
