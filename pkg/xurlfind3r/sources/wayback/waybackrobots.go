package wayback

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

func parseWaybackRobots(config *sources.Configuration, URL string) (robotsURLs chan string) {
	robotsURLs = make(chan string)

	robotsEntryRegex := regexp.MustCompile(`Disallow:\s?.+`)

	go func() {
		defer close(robotsURLs)

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

				disallowed := robotsEntryRegex.FindAllStringSubmatch(content, -1)

				if len(disallowed) < 1 {
					return
				}

				for index := range disallowed {
					entry := disallowed[index]

					temp := strings.Split(entry[0], "Disallow:")

					if len(temp) <= 1 {
						continue
					}

					robotsURL := strings.Trim(temp[1], " ")

					if robotsURL == "/" || robotsURL == "*" || robotsURL == "" {
						continue
					}

					robotsURL = strings.ReplaceAll(robotsURL, "*", "")

					for strings.HasPrefix(robotsURL, "/") {
						if len(robotsURL) >= 1 {
							robotsURL = robotsURL[1:] // Ex. /*/test or /*/*/demo
						} else {
							continue
						}
					}

					for strings.HasSuffix(robotsURL, "/") {
						if len(robotsURL) >= 1 {
							robotsURL = robotsURL[0 : len(robotsURL)-1]
						} else {
							continue
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
