//go:build windows

package main

import (
	"os"
	"testing"
)

func TestTerminationSignals(t *testing.T) {
	t.Parallel()

	signals := terminationSignals()

	if len(signals) != 1 {
		t.Fatalf("signals = %v, want 1 entry", signals)
	}

	if signals[0] != os.Interrupt {
		t.Fatalf("signals = %v, want [interrupt]", signals)
	}
}
