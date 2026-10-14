package input

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var domainPattern = regexp.MustCompile(`^(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`)

func ValidateDomain(domain string) (err error) {
	domain = strings.TrimSpace(domain)

	if domain == "" {
		err = errors.New("empty domain")

		return
	}

	if !domainPattern.MatchString(domain) {
		err = fmt.Errorf("invalid domain %q", domain)

		return
	}

	return
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
