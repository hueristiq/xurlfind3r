package wayback

import (
	"fmt"
	"mime"
	"regexp"
	"strings"
	"sync"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
)

func parseWaybackSource(config *sources.Configuration, domain, URL string, results chan sources.Result) {
	var err error

	var snapshots [][2]string

	snapshots, err = getSnapshots(URL)
	if err != nil {
		result := sources.Result{
			Type:   sources.Error,
			Source: "wayback:source",
			Error:  err,
		}

		results <- result

		return
	}

	lxExtractor := hqgourl.Extractor.Relaxed()

	var mdExtractor *regexp.Regexp

	mdExtractor, err = hqgourl.Extractor.ModerateMatchHost(`(\w[a-zA-Z0-9][a-zA-Z0-9-\\.]*\.)?` + regexp.QuoteMeta(domain))
	if err != nil {
		result := sources.Result{
			Type:   sources.Error,
			Source: "wayback:source",
			Error:  err,
		}

		results <- result

		return
	}

	regex1 := regexp.MustCompile(`^(//web\.archive\.org/web|https://web\.archive\.org/web|/web)/\d{14}([a-z]{2}_)?/.*`)
	regex2 := regexp.MustCompile(`^https?://.*`)

	wg := &sync.WaitGroup{}

	for _, row := range snapshots {
		wg.Add(1)

		go func(row [2]string) {
			defer wg.Done()

			content, err := getSnapshotContent(row)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: "wayback:source",
					Error:  err,
				}

				results <- result

				return
			}

			lxURLs := lxExtractor.FindAllString(content, -1)

			for _, lxURL := range lxURLs {
				lxURL = sources.FixURL(lxURL)

				// `/web/20230128054726/https://example.com/`
				// `//web.archive.org/web/20230128054726/https://example.com/`
				// `https://web.archive.org/web/20230128054726/https://example.com/`
				// `/web/20040111155853js_/http://example.com/2003/mm_menu.js`
				if regex1.MatchString(lxURL) {
					URLs := mdExtractor.FindAllString(lxURL, -1)

					for _, URL := range URLs {
						// `https://web.archive.org/web/20001110042700/mailto:info@safaricom.co.ke`->safaricom.co.ke
						if !strings.HasPrefix(URL, "http") {
							continue
						}

						if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
							continue
						}

						result := sources.Result{
							Type:   sources.URL,
							Source: "wayback:source",
							Value:  URL,
						}

						results <- result
					}

					continue
				}

				// `http://www.safaricom.co.ke/`
				// `https://web.archive.org/web/*/http://www.safaricom.co.ke/*`
				// `//html5shim.googlecode.com/svn/trunk/html5.js``
				if regex2.MatchString(lxURL) || strings.HasPrefix(lxURL, `//`) {
					URLs := mdExtractor.FindAllString(lxURL, -1)

					for _, URL := range URLs {
						if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
							continue
						}

						result := sources.Result{
							Type:   sources.URL,
							Source: "wayback:source",
							Value:  URL,
						}

						results <- result
					}

					continue
				}

				// text/javascript
				_, _, err := mime.ParseMediaType(lxURL)
				if err == nil {
					result := sources.Result{
						Type:   sources.Error,
						Source: "wayback:source",
						Error:  err,
					}

					results <- result

					continue
				}

				// `//archive.org/includes/analytics.js?v=c535ca67``
				// `archive.org/components/npm/lit/polyfill-support.js?v=c535ca67`
				// `archive.org/components/npm/@webcomponents/webcomponentsjs/webcomponents-bundle.js?v=c535ca67`
				// `archive.org/includes/build/js/ia-topnav.min.js?v=c535ca67`
				// `archive.org/includes/build/js/archive.min.js?v=c535ca67`
				// `archive.org/includes/build/css/archive.min.css?v=c535ca67`
				if strings.Contains(lxURL, "archive.org") {
					continue
				}

				parsedSourceURL, err := hqgourl.Parse(URL)
				if err != nil {
					result := sources.Result{
						Type:   sources.Error,
						Source: "wayback:source",
						Error:  err,
					}

					results <- result

					continue
				}

				lxURL = strings.TrimLeft(lxURL, "/")

				lxURL = fmt.Sprintf("%s://%s/%s", parsedSourceURL.Scheme, parsedSourceURL.Domain, lxURL)

				if !sources.IsInScope(lxURL, domain, config.IncludeSubdomains) {
					continue
				}

				result := sources.Result{
					Type:   sources.URL,
					Source: "wayback:source",
					Value:  lxURL,
				}

				results <- result
			}
		}(row)
	}

	wg.Wait()
}
