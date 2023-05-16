package wayback

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	hqurl "github.com/hueristiq/hqgoutils/url"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/valyala/fasthttp"
)

func parseWaybackRobots(URL string) (URLs chan string) {
	URLs = make(chan string)

	go func() {
		defer close(URLs)

		// retrieve snapshots
		var (
			err error
			res *fasthttp.Response
		)

		limiter.Wait()

		reqURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s&output=json&fl=timestamp,original&filter=statuscode:200&collapse=digest", URL)

		res, err = httpclient.SimpleGet(reqURL)
		if err != nil {
			return
		}

		if res.Header.ContentLength() == 0 {
			return
		}

		snapshots := [][2]string{}

		if err = json.Unmarshal(res.Body(), &snapshots); err != nil {
			return
		}

		if len(snapshots) < 2 {
			return
		}

		snapshots = snapshots[1:]

		// retrieve conteny
		wg := &sync.WaitGroup{}

		for _, row := range snapshots {
			wg.Add(1)

			go func(row [2]string) {
				defer wg.Done()

				var (
					err error
					res *fasthttp.Response
				)

				limiter.Wait()

				reqURL := fmt.Sprintf("https://web.archive.org/web/%sif_/%s", row[0], row[1])

				res, err = httpclient.SimpleGet(reqURL)
				if err != nil {
					return
				}

				content := string(res.Body())

				if content == "" {
					return
				}

				contentNotFoundFingerprint := "This page can't be displayed. Please use the correct URL address to access"

				if strings.Contains(content, contentNotFoundFingerprint) {
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
