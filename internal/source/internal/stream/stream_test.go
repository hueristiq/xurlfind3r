package stream_test

import (
	"context"
	"testing"
	"time"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

func TestEmit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan hqgosource.Result, 1)

	if !stream.Emit(ctx, results, hqgosource.Result{Value: "example.com"}) {
		t.Fatal("Emit rejected a live context")
	}

	if result := <-results; result.Value != "example.com" {
		t.Fatalf("result = %+v", result)
	}

	cancel()

	if stream.Emit(ctx, results, hqgosource.Result{Value: "ignored"}) {
		t.Fatal("Emit accepted a canceled context")
	}
}

func TestWait(t *testing.T) {
	t.Parallel()

	if err := stream.Wait(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := stream.Wait(ctx, time.Second); err == nil {
		t.Fatal("Wait accepted a canceled context")
	}
}
