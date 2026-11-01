package intelx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hueristiq/xurlfind3r/internal/source/intelx"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestSearch(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		target     intelx.Target
		wantTarget int
		wantTerm   string
		wantValue  string
	}{
		{name: "domains", target: intelx.Domains, wantTarget: 1, wantTerm: "example.com", wantValue: "a.example.com"},
		{name: "URLs", target: intelx.URLs, wantTarget: 3, wantTerm: "*example.com", wantValue: "https://a.example.com/x"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("k") != "secret" {
					writer.WriteHeader(http.StatusUnauthorized)

					return
				}

				writer.Header().Set("Content-Type", "application/json")

				switch r.URL.Path {
				case "/phonebook/search":
					var body struct {
						Term   string `json:"term"`
						Target int    `json:"target"`
					}

					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("decode search: %v", err)
					}

					if body.Term != test.wantTerm || body.Target != test.wantTarget {
						t.Errorf("search = %+v; want term %q target %d", body, test.wantTerm, test.wantTarget)
					}

					_, _ = writer.Write([]byte(`{"id":"search-id"}`))
				case "/phonebook/search/result":
					if r.URL.Query().Get("id") != "search-id" {
						t.Errorf("search ID = %q", r.URL.Query().Get("id"))
					}

					_, _ = writer.Write([]byte(`{"status":1,"selectors":[{"selectorvalue":"` + test.wantValue + `"},{"selectorvalue":"` + test.wantValue + `"}]}`))
				default:
					writer.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			host := strings.TrimPrefix(server.URL, "https://")

			client, err := intelx.New(server.Client(), host+":secret")
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

			if len(values) != 1 || values[0] != test.wantValue {
				t.Fatalf("values = %v; want one %q", values, test.wantValue)
			}
		})
	}
}

func TestSearchCustomPollPolicy(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/phonebook/search" {
			_, _ = writer.Write([]byte(`{"id":"search-id"}`))

			return
		}

		if polls.Add(1) == 1 {
			_, _ = writer.Write([]byte(`{"status":3,"selectors":[]}`))

			return
		}

		_, _ = writer.Write([]byte(`{"status":1,"selectors":[{"selectorvalue":"a.example.com"}]}`))
	}))
	defer server.Close()

	client, err := intelx.New(server.Client(), strings.TrimPrefix(server.URL, "https://")+":secret", intelx.WithPollPolicy(time.Millisecond, 2))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var values []string

	for result := range client.Search(context.Background(), "example.com", intelx.Domains) {
		if result.Err != nil {
			t.Fatalf("Search: %v", result.Err)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || polls.Load() != 2 {
		t.Fatalf("values = %v, polls = %d; want one value after two polls", values, polls.Load())
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()

	client := &http.Client{}
	for _, credential := range []string{"missing-separator", ":secret", "host:", "https://host:secret", "host/path:secret"} {
		if _, err := intelx.New(client, credential); err == nil {
			t.Errorf("New accepted credential %q", credential)
		}
	}

	if _, err := intelx.New(nil, "host:secret"); err == nil {
		t.Fatal("New accepted a nil HTTP client")
	}

	if _, err := intelx.New(client, "host:secret", intelx.WithPollPolicy(0, 0)); err == nil {
		t.Fatal("New accepted an invalid polling policy")
	}
}

func TestSearchRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	client, err := intelx.New(&http.Client{}, "host:secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, ok := <-client.Search(context.Background(), "example.com", 0)
	if !ok || result.Err == nil {
		t.Fatalf("invalid target result = %+v, %t; want an error", result, ok)
	}
}

func TestSearchRedactsCredential(t *testing.T) {
	t.Parallel()

	original := errors.New("transport failed with secret")
	doer := doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, original
	})

	client, err := intelx.New(doer, "host:secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result := <-client.Search(context.Background(), "example.com", intelx.Domains)
	if result.Err == nil || strings.Contains(result.Err.Error(), "secret") {
		t.Fatalf("error = %v; want redacted credential", result.Err)
	}

	if !errors.Is(result.Err, original) {
		t.Fatal("redacted error does not unwrap to the transport error")
	}
}

func TestSearchCancellationClosesChannel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := intelx.New(&http.Client{}, "host:secret")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for result := range client.Search(ctx, "example.com", intelx.Domains) {
		t.Fatalf("canceled search emitted %+v", result)
	}
}
