package wayback

import (
	"fmt"
	"mime"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
)

func parseWaybackSource(domain, URL string) (sourceURLs chan string) {
	sourceURLs = make(chan string)

	go func() {
		defer close(sourceURLs)

		var err error
		var snapshots [][2]string

		snapshots, err = getWaybackSnapshots(URL)
		if err != nil {
			return
		}

		lxExtractor := hqgourl.Extractor.Relaxed()

		var mdExtractor *regexp.Regexp

		mdExtractor, err = hqgourl.Extractor.ModerateMatchHost(`(\w[a-zA-Z0-9][a-zA-Z0-9-\\.]*\.)?` + regexp.QuoteMeta(domain))
		if err != nil {
			return
		}

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

				links := lxExtractor.FindAllString(content, -1)

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
						URLs := mdExtractor.FindAllString(sourceURL, -1)

						for _, URL := range URLs {
							sourceURLs <- URL
						}
					} else {
						_, _, err := mime.ParseMediaType(sourceURL)
						if err == nil {
							continue
						}

						URLs := mdExtractor.FindAllString(sourceURL, -1)

						for _, URL := range URLs {
							sourceURLs <- URL
						}

						if len(URLs) > 0 {
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
