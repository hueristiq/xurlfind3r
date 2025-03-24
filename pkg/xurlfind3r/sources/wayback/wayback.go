package wayback

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	hqgohttp "go.source.hueristiq.com/http"
	hqgolimiter "go.source.hueristiq.com/limiter"
)

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		var err error

		for page := uint(0); ; page++ {
			getURLsReqURL := "https://web.archive.org/cdx/search/cdx"
			getURLsReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"url":      "*." + domain + "/*",
					"output":   "json",
					"collapse": "urlkey",
					"fl":       "timestamp,original,mimetype,statuscode,digest",
					"pageSize": "100",
					"page":     fmt.Sprintf("%d", page),
				},
			}

			limiter.Wait()

			var getURLsRes *http.Response

			getURLsRes, err = hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				break
			}

			var getURLsResData [][]string

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				getURLsRes.Body.Close()

				break
			}

			getURLsRes.Body.Close()

			// check if there's results, wayback's pagination response
			// is not always correct when using a filter
			if len(getURLsResData) == 0 {
				break
			}

			getURLsResData = getURLsResData[1:]

			for _, record := range getURLsResData {
				URL := record[1]

				if !cfg.IsInScope(URL) {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}
		}
	}()

	return results
}

func (source *Source) Name() string {
	return sources.WAYBACK
}

var limiter = hqgolimiter.New(&hqgolimiter.Configuration{
	RequestsPerMinute:     40,
	MinimumDelayInSeconds: 30,
})
