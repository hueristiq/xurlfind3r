package wayback

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

func parseWaybackRobots(_ *sources.Configuration, URL string) (robotsURLs chan string) {
	robotsURLs = make(chan string)

	robotsEntryRegex := regexp.MustCompile(`(Allow|Disallow):\s?.+`)

	go func() {
		defer close(robotsURLs)

		snapshots, err := getWaybackSnapshots(URL)
		if err != nil {
			return
		}

		wg := &sync.WaitGroup{}

		for _, row := range snapshots {
			wg.Add(1)

			go func(row [2]string) {
				defer wg.Done()

				content, err := getWaybackContent(row)
				if err != nil {
					return
				}

				matches := robotsEntryRegex.FindAllStringSubmatch(content, -1)

				if len(matches) < 1 {
					return
				}

				for _, match := range matches {
					entry := match[0]

					temp := strings.Split(entry, ": ")

					if len(temp) <= 1 {
						continue
					}

					robotsURL := temp[1]

					if robotsURL == "/" || robotsURL == "*" || robotsURL == "" {
						continue
					}

					robotsURL = strings.ReplaceAll(robotsURL, "*", "")

					for strings.HasPrefix(robotsURL, "/") {
						if len(robotsURL) >= 1 {
							robotsURL = robotsURL[1:] // Ex. /*/test or /*/*/demo
						}
					}

					for strings.HasSuffix(robotsURL, "/") {
						if len(robotsURL) >= 1 {
							robotsURL = robotsURL[0 : len(robotsURL)-1]
						}
					}

					parsedURL, err := hqgourl.Parse(URL)
					if err != nil {
						continue
					}

					robotsURL = parsedURL.Scheme + "://" + filepath.Join(parsedURL.Domain, robotsURL)

					robotsURLs <- robotsURL
				}
			}(row)
		}

		wg.Wait()
	}()

	return
}
