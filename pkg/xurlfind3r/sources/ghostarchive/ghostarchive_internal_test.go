package ghostarchive

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

func TestSourceRunDecodesFixture(t *testing.T) {
	t.Parallel()

	page0, err := os.ReadFile("testdata/page0.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	page1, err := os.ReadFile("testdata/page1.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")

		switch r.URL.Query().Get("page") {
		case "", "0":
			writeResponse(t, w, string(page0))
		case "1":
			writeResponse(t, w, string(page1))
		default:
			t.Errorf("unexpected page: %q", r.URL.Query().Get("page"))

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
		"https://example.com/c",
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
