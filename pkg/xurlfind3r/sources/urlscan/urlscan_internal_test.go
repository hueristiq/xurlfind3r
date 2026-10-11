package urlscan

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/testutil"
)

func TestSourceRun(t *testing.T) {
	t.Parallel()

	t.Run("stops when the sort cursor is missing", func(t *testing.T) {
		t.Parallel()

		requests := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++

			w.Header().Set("Content-Type", "application/json")

			switch {
			case r.URL.Query().Get("search_after") == "":
				writeResponse(t, w, `{"results":[{"page":{"url":"https://example.com/a"},"sort":["1"]}],"status":200,"total":2,"has_more":true}`)
			case r.URL.Query().Get("search_after") == "1":
				writeResponse(t, w, `{"results":[{"page":{"url":"https://example.com/b"},"sort":null}],"status":200,"total":2,"has_more":true}`)
			default:
				t.Errorf("unexpected search_after: %q", r.URL.Query().Get("search_after"))

				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := &Source{
			keys: sources.Keys{"urlscan-test"},
		}

		cfg := testutil.NewConfiguration(t, server, "example.com")

		results := source.Run(context.Background(), cfg, "example.com")

		got := testutil.CollectURLs(t, results)

		if len(got) != 2 {
			t.Fatalf("got %v, want 2 URLs", got)
		}

		if requests != 2 {
			t.Fatalf("server saw %d requests, want 2 (the loop must not repeat the last page)", requests)
		}
	})
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
