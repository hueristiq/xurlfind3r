package sources

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

// Configuration holds the settings and parameters used by the Finder and its sources.
// It includes API keys for authenticating with external services and a compiled
// regular expression for parsing and validating domains.
//
// Fields:
// - Keys (Keys): Contains the API keys for various data sources.
// - IncludeSubdomains (bool): Whether subdomains should be included in the scope.
// - IsInScope (func(string) bool): Function to check if a URL is in scope.
//
// Example Usage:
//
//	cfg := Configuration{
//	    Keys: Keys{
//	        GitHub: []string{"key1", "key2"},
//	    },
//	    Extractor: regexp.MustCompile(`[a-z0-9.-]+\.[a-z]{2,}`),
//	}
type Configuration struct {
	Keys              Keys
	IncludeSubdomains bool
	IsInScope         func(URL string) (isInScope bool)
}

// Keys stores API keys for different data sources. Each data source has a `SourceKeys`
// field, which is a slice of strings. These keys are used for authentication when
// interacting with external APIs or services.
//
// Fields:
//   - Bevigil, Github, IntelX, etc. (SourceKeys): Slices of strings containing API keys
//     for their respective sources.
//
// Example Usage:
//
//	keys := Keys{
//	    GitHub: []string{"github-key-1", "github-key-2"},
//	    IntelX: []string{"intelx-key-1"},
//	}
type Keys struct {
	Bevigil    SourceKeys `yaml:"bevigil"`
	Github     SourceKeys `yaml:"github"`
	IntelX     SourceKeys `yaml:"intelx"`
	URLScan    SourceKeys `yaml:"urlscan"`
	VirusTotal SourceKeys `yaml:"virustotal"`
}

// SourceKeys is a slice of strings where each element represents an API key.
// This structure supports multiple keys for a single source to enable key rotation or fallback.
//
// Example Usage:
//
//	githubKeys := SourceKeys{"key1", "key2", "key3"}
type SourceKeys []string

// PickRandom selects and returns a random API key from the `SourceKeys` slice.
// If the slice is empty, it returns an error indicating that no keys are available.
//
// Parameters:
// - None
//
// Returns:
// - key (string): A randomly selected API key from the slice.
// - err (error): An error if the slice is empty or if randomness generation fails.
//
// Implementation Details:
//   - A cryptographically secure random number generator (`crypto/rand`) is used
//     to ensure a secure and unbiased selection.
//
// Example Usage:
//
//	keys := SourceKeys{"key1", "key2"}
//	selectedKey, err := keys.PickRandom()
//	if err != nil {
//	    fmt.Println("Error:", err)
//	} else {
//	    fmt.Println("Selected Key:", selectedKey)
//	}
func (k SourceKeys) PickRandom() (key string, err error) {
	length := len(k)

	if length == 0 {
		err = ErrNoKeys

		return
	}

	maximum := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, maximum)
	if err != nil {
		err = fmt.Errorf("failed to generate random index: %w", err)

		return
	}

	index := indexBig.Int64()

	key = k[index]

	return
}

// ErrNoKeys is a sentinel error returned when no API keys are available in a `SourceKeys` slice.
// This error is typically encountered when attempting to pick a random key from an empty slice.
//
// Example Usage:
//
//	keys := SourceKeys{}
//	selectedKey, err := keys.PickRandom()
//	if err == ErrNoKeys {
//	    fmt.Println("No keys available.")
//	}
var ErrNoKeys = errors.New("no keys available for the source")
