package zoomeye_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/zoomeye"
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

	for _, test := range []struct {
		name       string
		target     zoomeye.Target
		wantValues int
		wantFirst  string
	}{
		{name: "domains", target: zoomeye.Domains, wantValues: 4, wantFirst: "a.example.com"},
		{name: "URLs", target: zoomeye.URLs, wantValues: 2, wantFirst: "https://a.example.com/x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Api-Key") != "secret" {
					t.Error("missing API key")
				}

				var body struct {
					Query string `json:"qbase64"`
					Page  int    `json:"page"`
				}

				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
				}

				query, err := base64.StdEncoding.DecodeString(body.Query)
				if err != nil || string(query) != "*.example.com" {
					t.Errorf("query = %q, %v", query, err)
				}

				writer.Header().Set("Content-Type", "application/json")

				if body.Page == 1 {
					_, _ = writer.Write([]byte(`{"total":2,"data":[{"domain":"a.example.com","url":"https://a.example.com/x"}]}`))

					return
				}

				_, _ = writer.Write([]byte(`{"total":2,"data":[{"domain":"b.example.com","url":"https://b.example.com/y"}]}`))
			}))
			defer server.Close()

			client, err := zoomeye.New(testClient(server), "secret")
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			var values []string

			for result := range client.Search(context.Background(), "example.com", test.target) {
				if result.Err != nil {
					t.Fatalf("Search: %v", result.Err)
				}

				values = append(values, result.Value)
			}

			if len(values) != test.wantValues || values[0] != test.wantFirst {
				t.Fatalf("values = %v; want %d values starting with %q", values, test.wantValues, test.wantFirst)
			}
		})
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	if _, err := zoomeye.New(nil, "secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := zoomeye.New(&http.Client{}, ""); err == nil {
		t.Fatal("New accepted an empty API key")
	}
}

func TestSearchRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	client, err := zoomeye.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com", 0)
	if result.Err == nil {
		t.Fatal("Search accepted an invalid target")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := zoomeye.New(&http.Client{}, "secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com", zoomeye.Domains) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
