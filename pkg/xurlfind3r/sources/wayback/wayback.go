package wayback

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	hqratelimiter "github.com/hueristiq/hqgoutils/ratelimiter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type Source struct{}

var (
	limiter = hqratelimiter.New(&hqratelimiter.Options{
		RequestsPerMinute: 40,
	})
)

func (source *Source) Run(config sources.Configuration, domain string) (URLs chan sources.URL) {
	URLs = make(chan sources.URL)

	go func() {
		defer close(URLs)

		waybackURLs := make(chan string)

		go func() {
			defer close(waybackURLs)

			var (
				err     error
				results []string
			)

			if config.IncludeSubdomains {
				domain = "*." + domain
			}

			results, err = getWaybackURLs(domain)
			if err != nil {
				return
			}

			for index := range results {
				URL := results[index]
				if URL == "" {
					continue
				}

				waybackURLs <- URL
			}
		}()

		wg := &sync.WaitGroup{}
		robotsRegex := regexp.MustCompile(`^(https?)://[^ "]+/robots.txt$`)
		ignoreRegex := regexp.MustCompile(`(?i)\.(apng|bpm|png|bmp|gif|heif|ico|cur|jpg|jpeg|jfif|pjp|pjpeg|psd|raw|svg|tif|tiff|webp|xbm|3gp|aac|flac|mpg|mpeg|mp3|mp4|m4a|m4v|m4p|oga|ogg|ogv|mov|wav|webm|eot|woff|woff2|ttf|otf|css)(?:\?|#|$)`)

		for URL := range waybackURLs {
			wg.Add(1)

			go func(URL string) {
				defer wg.Done()

				URLs <- sources.URL{Source: source.Name(), Value: URL}

				if !config.ParseWaybackRobots && !config.ParseWaybackSource {
					return
				}

				if ignoreRegex.MatchString(URL) {
					return
				}

				if config.ParseWaybackRobots && robotsRegex.MatchString(URL) {
					for robotsURL := range parseWaybackRobots(URL) {
						URLs <- sources.URL{Source: source.Name() + ":robots", Value: robotsURL}
					}
				} else if config.ParseWaybackSource && !robotsRegex.MatchString(URL) {
					for sourceURL := range parseWaybackSource(URL) {
						URLs <- sources.URL{Source: source.Name() + ":source", Value: sourceURL}
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

	var (
		res *fasthttp.Response
	)

	limiter.Wait()

	reqURL := fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&output=txt&fl=original&collapse=urlkey", domain)

	res, err = httpclient.SimpleGet(reqURL)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(res.Body()))

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
	var (
		res *fasthttp.Response
	)

	limiter.Wait()

	reqURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=%s&output=json&fl=timestamp,original&collapse=digest", URL)

	res, err = httpclient.SimpleGet(reqURL)
	if err != nil {
		return
	}

	if res.Header.ContentLength() == 0 {
		return
	}

	if err = json.Unmarshal(res.Body(), &snapshots); err != nil {
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
		res       *fasthttp.Response
	)

	limiter.Wait()

	reqURL := fmt.Sprintf("https://web.archive.org/web/%sif_/%s", timestamp, URL)

	res, err = httpclient.SimpleGet(reqURL)
	if err != nil {
		return
	}

	content = string(res.Body())

	if content == "" {
		return
	}

	snapshotNotFoundFingerprint := "This page can't be displayed. Please use the correct URL address to access"

	if strings.Contains(content, snapshotNotFoundFingerprint) {
		err = fmt.Errorf(snapshotNotFoundFingerprint)

		return
	}

	return
}

func (source *Source) Name() string {
	return "wayback"
}
