package source_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
)

func TestDoer(t *testing.T) {
	t.Parallel()

	var client hqgosource.Doer = hqgosource.DoerFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.Context().Err(); err != nil {
			return nil, fmt.Errorf("source: request context: %w", err)
		}

		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	})

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d; want %d", response.StatusCode, http.StatusNoContent)
	}
}

func TestResult(t *testing.T) {
	t.Parallel()

	result := hqgosource.Result{Value: "example.com", Kind: "domain"}
	if result.Value != "example.com" || result.Kind != "domain" || result.Warning != "" || result.Err != nil {
		t.Fatalf("value result = %+v", result)
	}
}
