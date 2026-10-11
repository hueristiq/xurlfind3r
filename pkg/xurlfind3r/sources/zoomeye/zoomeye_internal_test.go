package zoomeye

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/testutil"
)

func TestSourceRunDecodesFixture(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/search.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if _, err := fmt.Fprint(w, string(fixture)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	source := &Source{
		keys: sources.Keys{"zoomeye-test"},
	}

	cfg := testutil.NewConfiguration(t, server, "example.com")

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
