package input

import (
	"os"

	hqgourlparser "github.com/hueristiq/hq-lib-url-go/parser"
)

// NormalizeDomain canonicalizes a domain or IP literal for use as a scanner
// target. Host validation and IDNA conversion live in hq-lib-url-go.
func NormalizeDomain(domain string) (normalized string, err error) {
	return hqgourlparser.NormalizeHost(domain) //nolint:wrapcheck // compatibility pass-through
}

func HasStdin() (has bool) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return
	}

	isPipe := (stat.Mode() & os.ModeCharDevice) == 0
	isFIFO := (stat.Mode() & os.ModeNamedPipe) != 0

	has = isPipe || isFIFO

	return
}
