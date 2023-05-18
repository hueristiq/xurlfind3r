package wayback

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var CT = map[string]bool{
	"text/html":              true,
	"application/json":       true,
	"image/jpeg":             true,
	"image/png":              true,
	"text/css":               true,
	"application/javascript": true,
	"text/javascript":        true,
	"text/template":          true,
	"core/embed":             true,
}
var URLRegex = regexp.MustCompile(`(?:"|')(((?:[a-zA-Z]{1,10}://|//)[^"'/]{1,}\.[a-zA-Z]{2,}[^"']{0,})|((?:/|\.\./|\./)[^"'><,;| *()(%%$^/\\\[\]][^"'><,;|()]{1,})|([a-zA-Z0-9_\-/]{1,}/[a-zA-Z0-9_\-/]{1,}\.(?:[a-zA-Z]{1,4}|action)(?:[\?|#][^"|']{0,}|))|([a-zA-Z0-9_\-/]{1,}/[a-zA-Z0-9_\-/]{3,}(?:[\?|#][^"|']{0,}|))|([a-zA-Z0-9_\-]{1,}\.(?:php|asp|aspx|jsp|json|action|html|js|txt|xml)(?:[\?|#][^"|']{0,}|)))(?:"|')`)

func parseWaybackSource(URL string) (URLs chan string) {
	URLs = make(chan string)

	parsedURL1, err := url.Parse(URL)
	if err != nil {
		return
	}

	// Create a regex to match URLs with specified host
	r := fmt.Sprintf(`http(s)?://` + parsedURL1.Hostname() + `/[^\s]*`)
	re := regexp.MustCompile(r)

	go func() {
		defer close(URLs)

		// retrieve snapshots
		var (
			err       error
			snapshots [][2]string
		)

		snapshots, err = getWaybackSnapshots(URL)
		if err != nil {
			return
		}

		// retrieve conteny
		wg := &sync.WaitGroup{}

		for _, row := range snapshots {
			wg.Add(1)

			go func(row [2]string) {
				defer wg.Done()

				var (
					err     error
					content string
				)

				content, err = getWaybackContent(row)
				if err != nil {
					return
				}

				for _, URL = range URLRegex.FindAllString(content, -1) {
					// remove beginning and ending quotes
					URL = strings.Trim(URL, "\"")
					URL = strings.Trim(URL, "'")

					// if URL starts with `//web.archive.org/web` append scheme i.e to process it as an absolute URL
					if strings.HasPrefix(URL, "//web.archive.org/web") {
						URL = "https:" + URL
					}

					parsedURL, err := url.Parse(URL)
					if err != nil {
						continue
					}

					if parsedURL.IsAbs() {
						matches := re.FindAllString(URL, -1)

						for _, match := range matches {
							URLs <- match
						}
					} else {
						// ignore content types
						if _, ignore := CT[URL]; ignore {
							continue
						}

						matches := re.FindAllString(URL, -1)

						for _, match := range matches {
							URLs <- match
						}

						if len(matches) == 1 {
							continue
						}

						// remove beginning slash
						URL = strings.TrimLeft(URL, "/")
						URL = fmt.Sprintf("%s://%s/%s", parsedURL1.Scheme, parsedURL1.Hostname(), URL)

						URLs <- URL
					}
				}
			}(row)
		}

		wg.Wait()
	}()

	return
}
