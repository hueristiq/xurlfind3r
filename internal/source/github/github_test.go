package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/github"
)

const (
	testSearchPath = "/search/code"
	testSecret     = "secret"
	testDomain     = "example.com"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testClient(server *httptest.Server) *http.Client {
	target, _ := url.Parse(server.URL)

	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		clone := request.Clone(request.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host

		return http.DefaultTransport.RoundTrip(clone)
	})}
}

func TestSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testSearchPath:
			if r.URL.Query().Get("q") != `testDomain` || r.URL.Query().Get("sort") != "created" || r.URL.Query().Get("order") != "asc" {
				t.Errorf("query = %q", r.URL.RawQuery)
			}

			if r.Header.Get("Authorization") != "token secret" || r.Header.Get("Accept") != "application/vnd.github.v3.text-match+json" {
				t.Error("missing GitHub headers")
			}

			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"items":[{"html_url":"https://github.com/org/repo/blob/main/file","text_matches":[{"fragment":"https://match.example.com"}]}]}`))
		case "/org/repo/main/file":
			_, _ = writer.Write([]byte("https://raw.example.com"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := github.New(testClient(server), []string{testSecret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var (
		values []string
		kinds  []string
	)

	for result := range client.Search(context.Background(), github.Query{Term: `testDomain`}) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
		kinds = append(kinds, result.Kind)
	}

	if len(values) != 2 || values[0] != "https://raw.example.com" || values[1] != "https://match.example.com" {
		t.Fatalf("values = %v; want raw content and text match", values)
	}

	if kinds[0] != github.Content || kinds[1] != github.Match {
		t.Fatalf("kinds = %v; want content then match", kinds)
	}
}

func TestSearchPagination(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testSearchPath:
			writer.Header().Set("Content-Type", "application/json")

			if r.URL.Query().Get("page") != "2" {
				writer.Header().Set("Link", `<https://api.github.com/search/code?page=2>; rel="next"`)
				_, _ = writer.Write([]byte(`{"items":[{"html_url":"https://github.com/org/repo/blob/main/first"}]}`))

				return
			}

			_, _ = writer.Write([]byte(`{"items":[{"html_url":"https://github.com/org/repo/blob/main/second"}]}`))
		case "/org/repo/main/first":
			_, _ = writer.Write([]byte("first"))
		case "/org/repo/main/second":
			_, _ = writer.Write([]byte("second"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := github.New(testClient(server), []string{testSecret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Search(context.Background(), github.Query{Term: testDomain}) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 2 || values[0] != "first" || values[1] != "second" {
		t.Fatalf("values = %v; want two linked pages", values)
	}
}

func TestSearchSkipsMatchesAfterRawFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testSearchPath {
			_, _ = writer.Write([]byte(`{"items":[{"html_url":"https://github.com/org/repo/blob/main/missing","text_matches":[{"fragment":"ignored"}]}]}`))

			return
		}

		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := github.New(testClient(server), []string{testSecret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(context.Background(), github.Query{Term: testDomain}) {
		if result.Value != "" {
			t.Fatalf("raw failure emitted %+v", result)
		}
	}
}

func TestSearchRateLimitRecovery(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writer.Header().Set("X-Ratelimit-Remaining", "0")
			writer.Header().Set("Retry-After", "1")
			writer.WriteHeader(http.StatusForbidden)

			return
		}

		_, _ = writer.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client, err := github.New(testClient(server), []string{testSecret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(context.Background(), github.Query{Term: testDomain}) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}
	}

	if requests.Load() != 2 {
		t.Fatalf("requests = %d; want a retry after recovery", requests.Load())
	}
}

func TestSearchRotatesKeys(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			if r.Header.Get("Authorization") != "token first" {
				t.Errorf("first credential = %q", r.Header.Get("Authorization"))
			}

			writer.Header().Set("X-Ratelimit-Remaining", "0")
			writer.Header().Set("Retry-After", "1")
			writer.WriteHeader(http.StatusForbidden)

			return
		}

		if r.Header.Get("Authorization") != "token second" {
			t.Errorf("second credential = %q", r.Header.Get("Authorization"))
		}

		_, _ = writer.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client, err := github.New(testClient(server), []string{"first", "second"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(context.Background(), github.Query{Term: testDomain}) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}
	}

	if requests.Load() != 2 {
		t.Fatalf("requests = %d; want a second key", requests.Load())
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := github.New(nil, []string{testSecret}); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := github.New(&http.Client{}, nil); err == nil {
		t.Fatal("New accepted no API keys")
	}

	if _, err := github.New(&http.Client{}, []string{""}); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := github.New(&http.Client{}, []string{testSecret})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, github.Query{Term: testDomain}) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
