package wayback

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	hqurl "github.com/hueristiq/hqgoutils/url"
)

func parseWaybackRobots(URL string) (URLs chan string) {
	URLs = make(chan string)

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

				pattern := regexp.MustCompile(`Disallow:\s?.+`)

				disallowed := pattern.FindAllStringSubmatch(content, -1)

				if len(disallowed) < 1 {
					return
				}

				for _, entry := range disallowed {
					temp := strings.Split(entry[0], "Disallow:")

					if len(temp) <= 1 {
						continue
					}

					endpoint := strings.Trim(temp[1], " ")

					if endpoint == "/" || endpoint == "*" || endpoint == "" {
						continue
					}

					endpoint = strings.ReplaceAll(endpoint, "*", "")

					for strings.HasPrefix(endpoint, "/") {
						if len(endpoint) >= 1 {
							endpoint = endpoint[1:] // Ex. /*/test or /*/*/demo
						} else {
							continue
						}
					}

					for strings.HasSuffix(endpoint, "/") {
						if len(endpoint) >= 1 {
							endpoint = endpoint[0 : len(endpoint)-1]
						} else {
							continue
						}
					}

					parsedURL, _ := hqurl.Parse(row[1])

					endpoint = filepath.Join(parsedURL.Host, endpoint)
					endpoint = parsedURL.Scheme + "://" + endpoint

					URLs <- endpoint
				}
			}(row)
		}

		wg.Wait()
	}()

	return
}
