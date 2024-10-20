package sources

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

// Configuration holds the overall settings for different data sources.
// It includes API keys for each source and flags for various parsing options.
type Configuration struct {
	IncludeSubdomains bool // Determines whether to include subdomains in the data collection process.

	Keys Keys // Holds API keys for multiple sources.

	IsInScope func(URL string) (isInScope bool)
}

// Keys holds API keys for different data sources, with each source having a set of API keys.
type Keys struct {
	Bevigil SourceKeys `yaml:"bevigil"`
	Github  SourceKeys `yaml:"github"`
	IntelX  SourceKeys `yaml:"intelx"`
	URLScan SourceKeys `yaml:"urlscan"`
}

// SourceKeys is a slice of strings representing API keys. Multiple API keys
// are used to allow for rotation or fallbacks when certain keys are unavailable.
type SourceKeys []string

// PickRandom selects and returns a random API key from the SourceKeys slice.
// If the slice is empty, an error is returned. It uses a cryptographically secure
// random number generator to ensure randomness.
func (k SourceKeys) PickRandom() (key string, err error) {
	length := len(k)

	// Return an error if no keys are available
	if length == 0 {
		err = ErrNoKeys

		return
	}

	// Generate a cryptographically secure random number within the range [0, length).
	maximum := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, maximum)
	if err != nil {
		err = fmt.Errorf("failed to generate random index: %w", err)

		return
	}

	// Convert the big integer index to a standard int64.
	index := indexBig.Int64()

	// Select the API key at the generated index.
	key = k[index]

	return
}

var ErrNoKeys = errors.New("no keys available for the source")
