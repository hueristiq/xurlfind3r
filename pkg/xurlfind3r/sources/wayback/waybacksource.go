package wayback

import (
	"fmt"
	"mime"
	"regexp"
	"strings"
	"sync"

	hqurl "github.com/hueristiq/hqgoutils/url"
)

func parseWaybackSource(URL string, URLsRegex *regexp.Regexp) (URLs chan string) {
	URLs = make(chan string)

	parsedURL, err := hqurl.Parse(URL)
	if err != nil {
		return
	}

	escapedDomain := regexp.QuoteMeta(parsedURL.ETLDPlusOne)
	pattern := fmt.Sprintf(`https?://([a-z0-9.-]*\.)?%s(/[a-zA-Z0-9()/*\-+_~:,.?#=]*)?`, escapedDomain)
	re := regexp.MustCompile(pattern)

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

		// retrieve content
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

				for _, sourceURL := range URLsRegex.FindAllString(content, -1) {
					// remove beginning and ending quotes
					sourceURL = strings.Trim(sourceURL, "\"")
					sourceURL = strings.Trim(sourceURL, "'")

					// remove beginning and ending spaces
					sourceURL = strings.Trim(sourceURL, " ")

					// if URL starts with `//web.archive.org/web` append scheme i.e to process it as an absolute URL
					if strings.HasPrefix(sourceURL, "//web.archive.org/web") {
						sourceURL = "https:" + sourceURL
					}

					parsedSourceURL, err := hqurl.Parse(sourceURL)
					if err != nil {
						continue
					}

					if parsedSourceURL.IsAbs() {
						matches := re.FindAllString(sourceURL, -1)

						for _, match := range matches {
							URLs <- match
						}
					} else {
						_, _, err := mime.ParseMediaType(sourceURL)
						if err == nil {
							continue
						}

						matches := re.FindAllString(sourceURL, -1)

						for _, match := range matches {
							URLs <- match
						}

						if len(matches) > 0 {
							continue
						}

						// remove beginning slash
						sourceURL = strings.TrimLeft(sourceURL, "/")

						sourceURL = fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Domain, sourceURL)

						URLs <- sourceURL
					}
				}
			}(row)
		}

		wg.Wait()
	}()

	return
}
