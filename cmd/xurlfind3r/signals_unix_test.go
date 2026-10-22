//go:build !windows

package main

import (
	"os"
	"syscall"
	"testing"
)

func TestTerminationSignals(t *testing.T) {
	t.Parallel()

	signals := terminationSignals()

	if len(signals) != 2 {
		t.Fatalf("signals = %v, want 2 entries", signals)
	}

	if signals[0] != os.Interrupt || signals[1] != syscall.SIGTERM {
		t.Fatalf("signals = %v, want [interrupt term]", signals)
	}
}
