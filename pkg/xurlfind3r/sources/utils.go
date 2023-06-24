package sources

import (
	"math/rand"
	"net/mail"
	"time"

	"github.com/hueristiq/hqgourl"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func PickRandom[T any](v []T) (picked T) {
	length := len(v)

	if length == 0 {
		return
	}

	picked = v[rand.Intn(length)]

	return
}

func IsValid(URL string) (isValid bool) {
	var err error

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
