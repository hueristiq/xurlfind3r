package wayback

import (
	"encoding/json"
	"fmt"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgolimiter "github.com/hueristiq/hq-go-limiter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
)

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.WAYBACK

	return
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		for page := uint(0); ; page++ {
			getURLsReqURL := "https://web.archive.org/cdx/search/cdx"
			getURLsReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"url":      "*." + domain + "/*",
					"output":   "json",
					"collapse": "urlkey",
					"fl":       "timestamp,original,mimetype,statuscode,digest",
					"pageSize": "100",
					"page":     cast.ToString(page),
				},
			}

			limiter.Wait()

			getURLsRes, err := hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				break
			}

			var getURLsResData [][]string

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
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

			// Slicing as [1:] to skip first result by default
			for _, record := range getURLsResData[1:] {
				var URL string

				var valid bool

				if URL, valid = cfg.Validator(record[1]); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: s.Name(),
					Value:  URL,
				}

				results <- result
			}
		}
	}()

	return results
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

var limiter = hqgolimiter.New(&hqgolimiter.Configuration{
	RequestsPerMinute:     40,
	MinimumDelayInSeconds: 30,
})

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
