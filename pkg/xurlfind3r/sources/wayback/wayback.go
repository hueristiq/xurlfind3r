package wayback

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
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
				err error
				res *fasthttp.Response
			)

			if config.IncludeSubdomains {
				domain = "*." + domain
			}

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

				waybackURLs <- URL
			}
		}()

		wg := &sync.WaitGroup{}
		robotsRegex := regexp.MustCompile(`^(https?)://[^ "]+/robots.txt$`)

		for URL := range waybackURLs {
			wg.Add(1)

			go func(URL string) {
				defer wg.Done()

				URLs <- sources.URL{Source: source.Name(), Value: URL}

				if robotsRegex.MatchString(URL) {
					for robotsURL := range parseWaybackRobots(URL) {
						URLs <- sources.URL{Source: source.Name() + ":robots", Value: robotsURL}
					}
				}
			}(URL)
		}

		wg.Wait()
	}()

	return
}

func (source *Source) Name() string {
	return "wayback"
}
