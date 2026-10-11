package virustotal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/testutil"
)

func TestMain(m *testing.M) {
	// The production limiter paces at 4 requests per minute; tests must
	// not wait on it.
	limiter = hqgolimiter.New(hqgolimiter.WithRate(60000, time.Minute))

	os.Exit(m.Run())
}

func TestSourceRunDecodesFixture(t *testing.T) {
	t.Parallel()

	fixture, err := os.ReadFile("testdata/report.json")
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
		keys: sources.Keys{"vt-test"},
	}

	cfg := testutil.NewConfiguration(t, server, "example.com")

	results := source.Run(context.Background(), cfg, "example.com")

	got := testutil.CollectURLs(t, results)

	want := []string{
		"https://example.com/a",
		"https://example.com/b",
		"https://sub.example.com",
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

func TestSourceRunRedactsKey(t *testing.T) {
	t.Parallel()

	source := &Source{
		keys: sources.Keys{"vt-secret"},
	}

	client, err := hqgohttp.NewClient(
		hqgohttp.WithHTTPClient(&http.Client{Transport: failingTransport{}}),
		hqgohttp.WithMaxRetries(0),
		hqgohttp.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	cfg := &sources.Configuration{
		Client: client,
	}

	results := source.Run(context.Background(), cfg, "example.com")

	errs := make([]error, 0)

	for result := range results {
		if result.Type != sources.ResultError {
			continue
		}

		errs = append(errs, result.Error)
	}

	if len(errs) != 1 {
		t.Fatalf("got %d error results, want 1", len(errs))
	}

	if strings.Contains(errs[0].Error(), "vt-secret") {
		t.Errorf("error leaks the key: %v", errs[0])
	}

	if !strings.Contains(errs[0].Error(), "[REDACTED]") {
		t.Errorf("error does not redact the key: %v", errs[0])
	}
}

// failingTransport fails every request with the request URL embedded in
// the error, mimicking a transport failure a source must redact.
type failingTransport struct{}

func (t failingTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	return nil, fmt.Errorf("dial tcp: %s", req.URL.String())
}
