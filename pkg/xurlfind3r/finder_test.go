package xurlfind3r_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r"
)

// TestNewConcurrent constructs finders from separate goroutines; the
// finders must not share source instances or any other mutable state.
func TestNewConcurrent(t *testing.T) {
	t.Parallel()

	wg := &sync.WaitGroup{}

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			finder, err := xurlfind3r.New(&xurlfind3r.Configuration{})
			if err != nil {
				t.Errorf("New: %v", err)

				return
			}

			if finder == nil {
				t.Errorf("New: got nil finder")
			}
		}()
	}

	wg.Wait()
}

// TestNewUnknownSource errors on unknown source names instead of
// silently running without them.
func TestNewUnknownSource(t *testing.T) {
	t.Parallel()

	_, err := xurlfind3r.New(&xurlfind3r.Configuration{
		SourcesToUse: []string{"wayback", "waybac"},
	})
	if err == nil {
		t.Fatalf("New: got nil error, want unknown source error")
	}

	if !strings.Contains(err.Error(), "unknown source") {
		t.Errorf("New: error = %v, want it to mention the unknown source", err)
	}
}

// TestNewDoesNotMutateConfiguration ensures New does not write into the
// caller's configuration.
func TestNewDoesNotMutateConfiguration(t *testing.T) {
	t.Parallel()

	cfg := &xurlfind3r.Configuration{
		SourcesToUse: []string{"otx"},
	}

	_, err := xurlfind3r.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(cfg.SourcesToUse) != 1 || cfg.SourcesToUse[0] != "otx" {
		t.Errorf("SourcesToUse = %v, want [otx]", cfg.SourcesToUse)
	}
}
