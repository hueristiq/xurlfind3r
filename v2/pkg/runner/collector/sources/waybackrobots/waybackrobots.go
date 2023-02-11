package waybackrobots

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgoutils/url"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/output"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/requests"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/sources"
)

type Source struct{}

func (source *Source) Run(keys sources.Keys, ftr filter.Filter) chan output.URL {
	domain := ftr.Domain

	URLs := make(chan output.URL)

	go func() {
		defer close(URLs)

		res, err := requests.SimpleGet(fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s/robots.txt&output=json&fl=timestamp,original&filter=statuscode:200&collapse=digest", domain))
		if err != nil {
			return
		}

		robotsURLsRows := [][2]string{}

		if err = json.Unmarshal(res.Body(), &robotsURLsRows); err != nil {
			return
		}

		if len(robotsURLsRows) < 2 {
			return
		}

		robotsURLsRows = robotsURLsRows[1:]

		wg := &sync.WaitGroup{}

		for _, row := range robotsURLsRows {
			wg.Add(1)

			go func(row [2]string) {
				defer wg.Done()

				res, err := requests.SimpleGet(fmt.Sprintf("https://web.archive.org/web/%sif_/%s", row[0], row[1]))
				if err != nil {
					return
				}

				pattern := regexp.MustCompile(`Disallow:\s?.+`)

				disallowed := pattern.FindAllStringSubmatch(string(res.Body()), -1)

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

					endpoint = strings.Replace(endpoint, "*", "", -1)

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

					parsedURL, _ := url.Parse(url.Options{URL: row[1]})

					endpoint = filepath.Join(parsedURL.Host, endpoint)
					endpoint = parsedURL.Scheme + "://" + endpoint

					if URL, ok := ftr.Examine(endpoint); ok {
						URLs <- output.URL{Source: source.Name(), Value: URL}
					}
				}
			}(row)
		}

		wg.Wait()
	}()

	return URLs
}

func (source *Source) Name() string {
	return "waybackrobots"
}
