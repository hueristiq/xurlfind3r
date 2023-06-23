package sources

import (
	"net/mail"

	"github.com/hueristiq/hqgourl"
)

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
