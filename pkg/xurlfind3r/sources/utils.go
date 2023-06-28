package sources

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/mail"

	"github.com/hueristiq/hqgourl"
)

func PickRandom[T any](v []T) (picked T, err error) {
	length := len(v)

	if length == 0 {
		return
	}

	// Generate a cryptographically secure random index
	max := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, max)
	if err != nil {
		err = fmt.Errorf("failed to generate random index: %v", err)

		return
	}

	index := indexBig.Int64()

	// Return the element at the random index
	picked = v[index]

	return
}

func IsValid(URL string) (isValid bool) {
	var (
		err error
	)

	_, err = hqgourl.Parse(URL)
	if err != nil {
		return
	}

	_, err = mail.ParseAddress(URL)
	if err == nil {
		return
	}

	isValid = true

	return
}

func IsInScope(URL, domain string, includeSubdomains bool) (isInScope bool) {
	parsedURL, err := hqgourl.Parse(URL)
	if err != nil {
		return
	}

	if !includeSubdomains &&
		parsedURL.Domain != domain &&
		parsedURL.Domain != "www."+domain {
		return
	}

	isInScope = true

	return
}
