package scraping

import (
	"strings"

	"github.com/enenumxela/urlx/pkg/urlx"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

func NormalizeURL(URL string, scope session.Scope) (string, bool) {
	URL = strings.Trim(URL, "\"")
	URL = strings.Trim(URL, "'")
	URL = strings.TrimRight(URL, "/")
	URL = strings.Trim(URL, " ")

	parsedURL, err := urlx.Parse(URL)
	if err != nil {
		return URL, false
	}

	if scope.FilterRegex.MatchString(parsedURL.Path) {
		return URL, false
	}

	if parsedURL.ETLDPlus1 == "" || parsedURL.ETLDPlus1 != scope.Domain {
		return URL, false
	}

	if !scope.IncludeSubdomains {
		if parsedURL.Host != scope.Domain && parsedURL.Host != "www."+scope.Domain {
			return URL, false
		}
	}

	return parsedURL.String(), true
}
