package input

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// HasStdin reports whether stdin carries piped input.
func HasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	isPipedFromChrDev := (stat.Mode() & os.ModeCharDevice) == 0
	isPipedFromFIFO := (stat.Mode() & os.ModeNamedPipe) != 0

	return isPipedFromChrDev || isPipedFromFIFO
}

// ValidateDomain trims domain and checks that it only contains hostname
// characters, so it can safely be interpolated into request URLs and
// joined into output file paths.
//
// Parameters:
//   - domain (string): the candidate domain.
//
// Returns:
//   - validated (string): the trimmed domain.
//   - err (error): non-nil when domain is empty or contains characters
//     outside letters, digits, dots, hyphens and underscores.
func ValidateDomain(domain string) (validated string, err error) {
	validated = strings.TrimSpace(domain)

	if validated == "" {
		return "", errors.New("domain is empty")
	}

	for index := range len(validated) {
		character := validated[index]

		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '.', character == '-', character == '_':
			continue
		default:
			return "", fmt.Errorf("domain %q contains invalid character %q", validated, character)
		}
	}

	return validated, nil
}
