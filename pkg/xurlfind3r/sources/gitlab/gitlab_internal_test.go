package gitlab

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

	t.Run("escapes the file path and ref", func(t *testing.T) {
		t.Parallel()

		rawRequests := make(chan string, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			switch r.URL.Path {
			case "/api/v4/search":
				writeResponse(t, w, `[{"data":"see https://example.com/x","project_id":123,"path":"dir/sub dir/file.go","ref":"feature/a&b"}]`)
			case "/api/v4/projects/123/repository/files/dir/sub dir/file.go/raw":
				rawRequests <- r.RequestURI

				writeResponse(t, w, "https://example.com/from-raw\n")
			default:
				t.Errorf("unexpected request path: %s", r.URL.Path)

				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		source := &Source{
			keys: sources.Keys{"glpat-test"},
		}

		cfg := testutil.NewConfiguration(t, server, "example.com")

		results := source.Run(context.Background(), cfg, "example.com")

		got := testutil.CollectURLs(t, results)

		if len(got) != 2 {
			t.Fatalf("got %v, want 2 URLs", got)
		}

		rawRequest := <-rawRequests

		wantRequest := "/api/v4/projects/123/repository/files/dir%2Fsub%20dir%2Ffile.go/raw?ref=feature%2Fa%26b"

		if rawRequest != wantRequest {
			t.Errorf("raw request = %q, want %q", rawRequest, wantRequest)
		}
	})
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := fmt.Fprint(w, body); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
