package github

import (
	"strings"
)

func getRawContentURL(URL string) (rawContentURL string) {
	rawContentURL = URL
	rawContentURL = strings.ReplaceAll(rawContentURL, "https://github.com/", "https://raw.githubusercontent.com/")
	rawContentURL = strings.ReplaceAll(rawContentURL, "/blob/", "/")

	return
}
