package sources

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/hueristiq/hqgourl"
)

func PickRandom[T any](v []T) (picked T, err error) {
	length := len(v)

	if length == 0 {
		return
	}

	max := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, max)
	if err != nil {
		err = fmt.Errorf("failed to generate random index: %v", err)

		return
	}

	index := indexBig.Int64()

	picked = v[index]

	return
}

func IsInScope(URL, domain string, includeSubdomains bool) (isInScope bool) {
	parsedURL, err := hqgourl.Parse(URL)
	if err != nil {
		return
	}

	parsedDomain, err := hqgourl.Parse(domain)
	if err != nil {
		return
	}

	if parsedURL.ETLDPlusOne != parsedDomain.ETLDPlusOne {
		return
	}

	if !includeSubdomains &&
		parsedURL.Domain != parsedDomain.Domain &&
		parsedURL.Domain != "www."+parsedDomain.Domain {
		return
	}

	isInScope = true

	return
}
