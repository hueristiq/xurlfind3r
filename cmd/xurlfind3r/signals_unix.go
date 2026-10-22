//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func ignoreBrokenPipe() {
	signal.Ignore(syscall.SIGPIPE)
}

func terminationSignals() (signals []os.Signal) {
	signals = []os.Signal{os.Interrupt, syscall.SIGTERM}

	return
}
