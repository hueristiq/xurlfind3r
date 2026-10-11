package virustotal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

func TestMain(m *testing.M) {
	// The production limiter paces at 4 requests per minute; tests must
	// not wait on it.
	limiter = hqgolimiter.New(hqgolimiter.WithRate(60000, time.Minute))

	os.Exit(m.Run())
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
