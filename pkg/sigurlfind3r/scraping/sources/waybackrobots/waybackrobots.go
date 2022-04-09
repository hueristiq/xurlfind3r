package waybackrobots

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/enenumxela/urlx/pkg/urlx"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

type Source struct{}

func (source *Source) Run(domain string, ses *session.Session, includeSubs bool) chan scraping.URL {
	URLs := make(chan scraping.URL)

	go func() {
		defer close(URLs)

		res, err := ses.SimpleGet(fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s/robots.txt&output=json&fl=timestamp,original&filter=statuscode:200&collapse=digest", domain))
		if err != nil {
			ses.DiscardHTTPResponse(res)
			return
		}

		defer res.Body.Close()

		robotsURLsRows := [][2]string{}

		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return
		}

		if err = json.Unmarshal(data, &robotsURLsRows); err != nil {
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

				res, err := ses.SimpleGet(fmt.Sprintf("https://web.archive.org/web/%sif_/%s", row[0], row[1]))
				if err != nil {
					ses.DiscardHTTPResponse(res)
					return
				}

				buf := new(bytes.Buffer)
				buf.ReadFrom(res.Body)

				pattern := regexp.MustCompile(`Disallow:\s?.+`)

				disallowed := pattern.FindAllStringSubmatch(buf.String(), -1)

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

					parsedURL, _ := urlx.Parse(row[1])

					endpoint = filepath.Join(parsedURL.Host, endpoint)
					endpoint = parsedURL.Scheme + "://" + endpoint

					if URL, ok := scraping.NormalizeURL(endpoint, ses.Scope); ok {
						URLs <- scraping.URL{Source: source.Name(), Value: URL}
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
