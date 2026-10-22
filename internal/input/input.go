package input

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

func NormalizeDomain(domain string) (normalized string, err error) {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")

	if domain == "" {
		err = errors.New("empty domain")

		return
	}

	if !utf8.ValidString(domain) {
		err = errors.New("domain is not valid UTF-8")

		return
	}

	normalized, err = idna.Lookup.ToASCII(domain)
	if err != nil {
		err = fmt.Errorf("invalid domain %q: %w", domain, err)

		return
	}

	normalized = strings.ToLower(normalized)

	if len(normalized) > maxDomainLength {
		err = fmt.Errorf("invalid domain %q: exceeds %d bytes", domain, maxDomainLength)

		return
	}

	labels := strings.Split(normalized, ".")

	for _, label := range labels {
		if labelErr := validateLabel(label); labelErr != nil {
			err = fmt.Errorf("invalid domain %q: %w", domain, labelErr)

			return
		}
	}

	return
}

func validateLabel(label string) (err error) {
	if label == "" {
		err = errors.New("empty label")

		return
	}

	if len(label) > maxLabelLength {
		err = fmt.Errorf("label %q exceeds %d bytes", label, maxLabelLength)

		return
	}

	for index := range len(label) {
		character := label[index]

		switch {
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '-':
		default:
			err = fmt.Errorf("invalid character %q in label %q", character, label)

			return
		}
	}

	if label[0] == '-' || label[len(label)-1] == '-' {
		err = fmt.Errorf("label %q starts or ends with a hyphen", label)

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
