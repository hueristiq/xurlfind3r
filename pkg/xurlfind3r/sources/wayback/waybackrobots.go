package wayback

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

func parseWaybackRobots(_ *sources.Configuration, URL string, results chan sources.Result) {
	robotsEntryRegex := regexp.MustCompile(`(Allow|Disallow):\s?.+`)

	snapshots, err := getSnapshots(URL)
	if err != nil {
		result := sources.Result{
			Type:   sources.Error,
			Source: "wayback:robots",
			Error:  err,
		}

		results <- result

		return
	}

	wg := &sync.WaitGroup{}

	for _, row := range snapshots {
		wg.Add(1)

		go func(row [2]string) {
			defer wg.Done()

			content, err := getSnapshotContent(row)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: "wayback:robots",
					Error:  err,
				}

				results <- result

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
					result := sources.Result{
						Type:   sources.Error,
						Source: "wayback:robots",
						Error:  err,
					}

					results <- result

					continue
				}

				robotsURL = parsedURL.Scheme + "://" + filepath.Join(parsedURL.Domain, robotsURL)

				result := sources.Result{
					Type:   sources.URL,
					Source: "wayback:robots",
					Value:  robotsURL,
				}

				results <- result
			}
		}(row)
	}

	wg.Wait()
}
