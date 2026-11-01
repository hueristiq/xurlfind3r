package gitlab_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/gitlab"
)

const (
	testSearchPath = "/api/v4/search"
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
		if r.Header.Get("Private-Token") != testSecret {
			t.Error("missing private token")
		}

		switch r.URL.Path {
		case testSearchPath:
			if r.URL.Query().Get("scope") != "blobs" || r.URL.Query().Get("search") != testDomain {
				t.Errorf("query = %q", r.URL.RawQuery)
			}

			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`[{"data":"https://snippet.example.com","project_id":7,"path":"dir/file","ref":"main"}]`))
		case "/api/v4/projects/7/repository/files/dir/file/raw":
			if r.URL.Query().Get("ref") != "main" {
				t.Errorf("ref = %q", r.URL.Query().Get("ref"))
			}

			_, _ = writer.Write([]byte("https://raw.example.com"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := gitlab.New(testClient(server), testSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var (
		values []string
		kinds  []string
	)

	for result := range client.Search(context.Background(), testDomain) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
		kinds = append(kinds, result.Kind)
	}

	if len(values) != 2 || values[0] != "https://snippet.example.com" || values[1] != "https://raw.example.com" {
		t.Fatalf("values = %v; want snippet and raw content", values)
	}

	if kinds[0] != gitlab.Snippet || kinds[1] != gitlab.Content {
		t.Fatalf("kinds = %v; want snippet then content", kinds)
	}
}

func TestSearchSkipsFailedRawContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testSearchPath {
			_, _ = writer.Write([]byte(`[{"data":"snippet","project_id":7,"path":"missing","ref":"main"}]`))

			return
		}

		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := gitlab.New(testClient(server), testSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Search(context.Background(), testDomain) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "snippet" {
		t.Fatalf("values = %v; want only the search snippet", values)
	}
}

func TestSearchWarnsOnNonadvancingPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testSearchPath {
			writer.Header().Set("X-Next-Page", "1")
			_, _ = writer.Write([]byte(`[]`))

			return
		}

		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := gitlab.New(testClient(server), testSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), testDomain)
	if result.Warning == "" {
		t.Fatalf("result = %+v; want a nonadvancing-page warning", result)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := gitlab.New(nil, testSecret); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := gitlab.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty private token")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := gitlab.New(&http.Client{}, testSecret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, testDomain) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
