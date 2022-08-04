package scraping

import (
	"strings"

	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/session"
	"github.com/hueristiq/url"
)

func NormalizeURL(URL string, scope session.Scope) (string, bool) {
	URL = strings.Trim(URL, "\"")
	URL = strings.Trim(URL, "'")
	URL = strings.TrimRight(URL, "/")
	URL = strings.Trim(URL, " ")

	parsedURL, err := url.Parse(url.Options{URL: URL})
	if err != nil {
		return URL, false
	}

	if scope.FilterRegex != nil {
		if scope.FilterRegex.MatchString(parsedURL.Path) {
			return URL, false
		}
	}

	if parsedURL.ETLDPlusOne == "" || parsedURL.ETLDPlusOne != scope.Domain {
		return URL, false
	}

	if !scope.IncludeSubdomains {
		if parsedURL.Host != scope.Domain || parsedURL.Host != "www."+scope.Domain {
			return URL, false
		}
	}

	return parsedURL.String(), true
}
