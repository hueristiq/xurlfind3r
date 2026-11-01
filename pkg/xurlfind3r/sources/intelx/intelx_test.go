package intelx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
)

func TestRunMapsURLs(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/phonebook/search" {
			_, _ = writer.Write([]byte(`{"id":"search-id"}`))

			return
		}

		_, _ = writer.Write([]byte(`{"status":1,"selectors":[{"selectorvalue":"https://api.example.com/path"}]}`))
	}))
	defer server.Close()

	client, err := hqgohttp.New(hqgohttp.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New HTTP client: %v", err)
	}

	source := intelx.New()
	source.UseKeys(strings.TrimPrefix(server.URL, "https://") + ":secret")

	cfg := &sources.Configuration{
		Client:    client,
		Validator: func(value string) (string, bool) { return value, true },
	}

	var values []string

	for result := range source.Run(context.Background(), cfg, "example.com") {
		if result.Error != nil {
			t.Fatalf("Run: %v", result.Error)
		}

		if result.Type != sources.ResultURL {
			t.Fatalf("result type = %v; want URL", result.Type)
		}

		values = append(values, result.Value)
	}

	if len(values) != 1 || values[0] != "https://api.example.com/path" {
		t.Fatalf("values = %v; want the provider URL", values)
	}
}

func TestRunRequiresCredential(t *testing.T) {
	t.Parallel()

	source := intelx.New()
	results := source.Run(context.Background(), &sources.Configuration{}, "example.com")

	result, ok := <-results
	if !ok || result.Error == nil {
		t.Fatalf("missing-credential result = %+v, %t; want an error", result, ok)
	}
}
