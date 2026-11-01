// Package stream implements the shared result-channel cancellation policy.
package stream

import (
	"context"
	"fmt"
	"time"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
)

// Emit sends result unless the search context has ended.
func Emit(ctx context.Context, results chan<- hqgosource.Result, result hqgosource.Result) bool {
	if ctx.Err() != nil {
		return false
	}

	select {
	case results <- result:
		return true
	case <-ctx.Done():
		return false
	}
}

// Wait waits for duration or context cancellation.
func Wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("source: wait interrupted: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
