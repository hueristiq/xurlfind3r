package filter

import (
	"regexp"
	"strings"

	hqurl "github.com/hueristiq/hqgoutils/url"
)

type Filter struct {
	Domain            string
	IncludeSubdomains bool
	ExcludeRegex      *regexp.Regexp
}

func (filter Filter) Examine(inputURL string) (outputURL string, pass bool) {
	var (
		err       error
		parsedURL *hqurl.URL
	)

	outputURL = inputURL

	outputURL = strings.Trim(outputURL, "\"")
	outputURL = strings.Trim(outputURL, "'")
	outputURL = strings.TrimRight(outputURL, "/")
	outputURL = strings.Trim(outputURL, " ")

	// if error parsing, ignore URL
	parsedURL, err = hqurl.Parse(hqurl.Options{URL: outputURL})
	if err != nil {
		return
	}

	// if not under the domain, ignore
	if !strings.HasSuffix(parsedURL.Host, filter.Domain) {
		return
	}

	// if !IncludeSubdomains and is subdomains, ignore
	if !filter.IncludeSubdomains &&
		parsedURL.Host != filter.Domain &&
		parsedURL.Host != "www."+filter.Domain {
		return
	}

	// if matches ignore patter, ignore
	if filter.ExcludeRegex != nil &&
		filter.ExcludeRegex.MatchString(parsedURL.Path) {
		return
	}

	pass = true

	return
}
