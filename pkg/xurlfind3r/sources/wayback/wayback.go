package wayback

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgolimit"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type Source struct{}

var (
	limiter = hqgolimit.New(&hqgolimit.Options{
		RequestsPerMinute: 40,
	})
)

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		waybackURLs := make(chan string)

		go func() {
			defer close(waybackURLs)

			if config.IncludeSubdomains {
				domain = "*." + domain
			}

			var err error

			var URLs []string

			URLs, err = getWaybackURLs(domain)
			if err != nil {
				return
			}

			for _, URL := range URLs {
				waybackURLs <- URL
			}
		}()

		mediaURLRegex := regexp.MustCompile(`(?i)\.(apng|bpm|png|bmp|gif|heif|ico|cur|jpg|jpeg|jfif|pjp|pjpeg|psd|raw|svg|tif|tiff|webp|xbm|3gp|aac|flac|mpg|mpeg|mp3|mp4|m4a|m4v|m4p|oga|ogg|ogv|mov|wav|webm|eot|woff|woff2|ttf|otf)(?:\?|#|$)`)
		robotsURLsRegex := regexp.MustCompile(`^(https?)://[^ "]+/robots.txt$`)

		wg := &sync.WaitGroup{}

		for URL := range waybackURLs {
			wg.Add(1)

			go func(URL string) {
				defer wg.Done()

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}

				if !config.ParseWaybackRobots && !config.ParseWaybackSource {
					return
				}

				if mediaURLRegex.MatchString(URL) {
					return
				}

				if config.ParseWaybackRobots &&
					robotsURLsRegex.MatchString(URL) {
					for robotsURL := range parseWaybackRobots(config, URL) {
						if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
							continue
						}

						URLsChannel <- sources.URL{Source: source.Name() + ":robots", Value: robotsURL}
					}
				} else if config.ParseWaybackSource &&
					!robotsURLsRegex.MatchString(URL) {
					for sourceURL := range parseWaybackSource(domain, URL) {
						if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
							continue
						}

						URLsChannel <- sources.URL{Source: source.Name() + ":source", Value: sourceURL}
					}
				}
			}(URL)
		}

		wg.Wait()
	}()

	return
}

func getWaybackURLs(domain string) (URLs []string, err error) {
	URLs = []string{}

	limiter.Wait()

	getURLsReqURL := fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&output=txt&fl=original&collapse=urlkey", domain)

	var getURLsRes *fasthttp.Response

	getURLsRes, err = httpclient.SimpleGet(getURLsReqURL)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(getURLsRes.Body()))

	for scanner.Scan() {
		URL := scanner.Text()
		if URL == "" {
			continue
		}

		URLs = append(URLs, URL)
	}

	if err = scanner.Err(); err != nil {
		return
	}

	return
}

func getWaybackSnapshots(URL string) (snapshots [][2]string, err error) {
	limiter.Wait()

	getSnapshotsReqURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s&output=json&fl=timestamp,original&collapse=digest", URL)

	var getSnapshotsRes *fasthttp.Response

	getSnapshotsRes, err = httpclient.SimpleGet(getSnapshotsReqURL)
	if err != nil {
		return
	}

	if getSnapshotsRes.Header.ContentLength() == 0 {
		return
	}

	if err = json.Unmarshal(getSnapshotsRes.Body(), &snapshots); err != nil {
		return
	}

	if len(snapshots) < 2 {
		return
	}

	snapshots = snapshots[1:]

	return
}

func getWaybackContent(snapshot [2]string) (content string, err error) {
	var (
		timestamp = snapshot[0]
		URL       = snapshot[1]
	)

	limiter.Wait()

	getSnapshotContentReqURL := fmt.Sprintf("https://web.archive.org/web/%sif_/%s", timestamp, URL)

	var getSnapshotContentRes *fasthttp.Response

	getSnapshotContentRes, err = httpclient.SimpleGet(getSnapshotContentReqURL)
	if err != nil {
		return
	}

	content = string(getSnapshotContentRes.Body())

	if content == "" {
		return
	}

	snapshotNotFoundFingerprint := "This page can't be displayed. Please use the correct URL address to access"

	if strings.Contains(content, snapshotNotFoundFingerprint) {
		err = fmt.Errorf("%s", snapshotNotFoundFingerprint)

		return
	}

	return
}

func (source *Source) Name() string {
	return "wayback"
}
