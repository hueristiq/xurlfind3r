package wayback

import (
	"encoding/json"
	"fmt"
	"net/http"

	hqgolimiter "github.com/hueristiq/hq-go-limiter"
	hqgourl "github.com/hueristiq/hq-go-url"
	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		var err error

		for page := uint(0); ; page++ {
			getURLsReqURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&collapse=urlkey&fl=timestamp,original,mimetype,statuscode,digest&pageSize=100&page=%d", domain, page)

			limiter.Wait()

			var getURLsRes *http.Response

			getURLsRes, err = httpclient.SimpleGet(getURLsReqURL)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				httpclient.DiscardResponse(getURLsRes)

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

var up = hqgourl.NewParser(hqgourl.ParserWithDefaultScheme("http"))
