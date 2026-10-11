package commoncrawl

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
	// The production limiter paces at 30 requests per minute; tests must
	// not wait on it.
	limiter = hqgolimiter.New(hqgolimiter.WithRate(60000, time.Minute))

	os.Exit(m.Run())
}

const testIndexPath = "/CC-MAIN-2026-40-index"

func TestSourceRun(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/collinfo.json":
			writeResponse(t, w, `[{"id":"CC-MAIN-2026-40","cdx-api":"https://index.commoncrawl.org/CC-MAIN-2026-40-index"}]`)
		case r.URL.Path == testIndexPath && r.URL.Query().Get("showNumPages") == "true":
			writeResponse(t, w, `{"blocks":1,"pageSize":5,"pages":2}`)
		case r.URL.Path == testIndexPath && r.URL.Query().Get("page") == "0":
			writeResponse(t, w, "{\"url\":\"https://example.com/a\"}\n{\"url\":\"https://example.com.evil.com/phantom\"}\n")
		case r.URL.Path == testIndexPath && r.URL.Query().Get("page") == "1":
			writeResponse(t, w, "{\"url\":\"https://sub.example.com/b\"}\n")
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())

			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := testutil.NewConfiguration(t, server, "example.com")

	source := &Source{}

	results := source.Run(context.Background(), cfg, "example.com")

	got := testutil.CollectURLs(t, results)

	want := []string{
		"https://example.com/a",
		"https://sub.example.com/b",
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
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
