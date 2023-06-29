package wayback

import (
	"fmt"
	"mime"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

func parseWaybackSource(config *sources.Configuration, URL string) (sourceURLs chan string) {
	sourceURLs = make(chan string)

	go func() {
		defer close(sourceURLs)

		// retrieve snapshots
		snapshots, err := getWaybackSnapshots(URL)
		if err != nil {
			return
		}

		// retrieve and parse snapshots' content for robotsURLs
		wg := &sync.WaitGroup{}

		for index := range snapshots {
			row := snapshots[index]

			wg.Add(1)

			go func(row [2]string) {
				defer wg.Done()

				content, err := getWaybackContent(row)
				if err != nil {
					return
				}

				links := config.LinkFinderRegex.FindAllString(content, -1)

				for index := range links {
					sourceURL := links[index]
					// remove beginning and ending quotes
					sourceURL = strings.Trim(sourceURL, "\"")
					sourceURL = strings.Trim(sourceURL, "'")

					// remove beginning and ending spaces
					sourceURL = strings.Trim(sourceURL, " ")

					// if URL starts with `//web.archive.org/web` append scheme i.e to process it as an absolute URL
					if strings.HasPrefix(sourceURL, "//web.archive.org/web") {
						sourceURL = "https:" + sourceURL
					}

					parsedSourceURL, err := hqgourl.Parse(sourceURL)
					if err != nil {
						continue
					}

					if parsedSourceURL.IsAbs() {
						matches := config.URLsRegex.FindAllString(sourceURL, -1)

						for _, match := range matches {
							sourceURLs <- match
						}
					} else {
						_, _, err := mime.ParseMediaType(sourceURL)
						if err == nil {
							continue
						}

						matches := config.URLsRegex.FindAllString(sourceURL, -1)

						for _, match := range matches {
							sourceURLs <- match
						}

						if len(matches) > 0 {
							continue
						}

						// remove beginning slash
						sourceURL = strings.TrimLeft(sourceURL, "/")

						sourceURL = fmt.Sprintf("%s://%s/%s", parsedSourceURL.Scheme, parsedSourceURL.Domain, sourceURL)

						sourceURLs <- sourceURL
					}
				}
			}(row)
		}

		wg.Wait()
	}()

	return
}
