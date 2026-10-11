package wayback

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/testutil"
)

func TestMain(m *testing.M) {
	// The production limiter paces at 40 requests per minute; tests must
	// not wait on it.
	limiter = hqgolimiter.New(hqgolimiter.WithRate(60000, time.Minute))

	os.Exit(m.Run())
}

func TestGetPage(t *testing.T) {
	t.Parallel()

	t.Run("decodes a page", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("pageSize"); got != "10000" {
				t.Errorf("pageSize = %q, want %q", got, "10000")
			}

			w.Header().Set("Content-Type", "application/json")

			writeBody(t, w, `[["timestamp","original"],["20230101","https://example.com/x"],["20230102","https://sub.example.com/y"]]`)
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		data, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err != nil {
			t.Fatalf("getPage: %v", err)
		}

		if len(data) != 3 {
			t.Fatalf("got %d records, want 3", len(data))
		}
	})

	t.Run("empty body is clean end-of-results", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		data, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err != nil {
			t.Fatalf("getPage: %v", err)
		}

		if data != nil {
			t.Fatalf("got %v, want nil", data)
		}
	})

	t.Run("whitespace body is clean end-of-results", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			writeBody(t, w, " \n\t ")
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		data, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err != nil {
			t.Fatalf("getPage: %v", err)
		}

		if data != nil {
			t.Fatalf("got %v, want nil", data)
		}
	})

	t.Run("malformed body is an error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			writeBody(t, w, "not json")
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		_, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err == nil {
			t.Fatalf("getPage: got nil error, want error")
		}
	})

	t.Run("429 waits and resumes the same page", func(t *testing.T) {
		t.Parallel()

		requests := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++

			if requests == 1 {
				w.WriteHeader(http.StatusTooManyRequests)

				return
			}

			w.Header().Set("Content-Type", "application/json")

			writeBody(t, w, `[["timestamp","original"],["20230101","https://example.com/x"]]`)
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		data, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err != nil {
			t.Fatalf("getPage: %v", err)
		}

		if len(data) != 2 {
			t.Fatalf("got %d records, want 2", len(data))
		}

		if requests != 2 {
			t.Fatalf("server saw %d requests, want 2", requests)
		}
	})

	t.Run("429 beyond the retry cap is an error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		_, err := getPage(context.Background(), cfg, "example.com", 0, time.Millisecond)
		if err == nil {
			t.Fatalf("getPage: got nil error, want error")
		}
	})
}

func TestSourceRun(t *testing.T) {
	t.Parallel()

	t.Run("paginates until a header-only page", func(t *testing.T) {
		t.Parallel()

		pageRequests := make(chan string, 4)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageRequests <- r.URL.Query().Get("page")

			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Query().Get("page") {
			case "0":
				writeBody(t, w, `[["timestamp","original"],["20230101","https://example.com/a"],["20230102","https://example.com.evil.com/phantom"]]`)
			case "1":
				writeBody(t, w, `[["timestamp","original"]]`)
			default:
				writeBody(t, w, `[["timestamp","original"],["20230201","https://example.com/b"]]`)
			}
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		source := &Source{}

		results := source.Run(context.Background(), cfg, "example.com")

		got := testutil.CollectURLs(t, results)

		want := []string{
			"https://example.com/a",
		}

		slices.Sort(got)
		slices.Sort(want)

		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}

		first := <-pageRequests
		second := <-pageRequests

		if first != "0" || second != "1" {
			t.Errorf("pages requested = %q, %q; want 0, 1", first, second)
		}
	})

	t.Run("stops cleanly on an empty page", func(t *testing.T) {
		t.Parallel()

		requests := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++

			w.Header().Set("Content-Type", "application/json")

			switch requests {
			case 1:
				writeBody(t, w, `[["timestamp","original"],["20230101","https://example.com/a"]]`)
			default:
				writeBody(t, w, "")
			}
		}))
		defer server.Close()

		cfg := testutil.NewConfiguration(t, server, "example.com")

		source := &Source{}

		results := source.Run(context.Background(), cfg, "example.com")

		got := testutil.CollectURLs(t, results)

		if len(got) != 1 || got[0] != "https://example.com/a" {
			t.Fatalf("got %v, want [https://example.com/a]", got)
		}

		if requests != 2 {
			t.Fatalf("server saw %d requests, want 2", requests)
		}
	})
}

func writeBody(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
