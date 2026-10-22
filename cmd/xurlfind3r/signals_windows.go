//go:build windows

package main

import "os"

func ignoreBrokenPipe() {}

func terminationSignals() (signals []os.Signal) {
	signals = []os.Signal{os.Interrupt}

	return
}
