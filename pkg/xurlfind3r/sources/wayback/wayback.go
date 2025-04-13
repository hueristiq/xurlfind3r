// Package wayback provides an implementation of the sources.Source interface
// for interacting with the Wayback Machine API.
//
// The Wayback Machine API (via Common Crawl's CDX server) allows retrieving historical
// snapshots of URLs for a given domain. This package defines a Source type that implements
// the Run and Name methods as specified by the sources.Source interface. The Run method
// queries the Wayback Machine API for URL snapshots matching a target domain, validates
// the retrieved URLs using the provided configuration, and streams valid URLs or errors via a channel.
package wayback

import (
	"encoding/json"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgolimiter "github.com/hueristiq/hq-go-limiter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
)

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the Wayback Machine API.
type Source struct{}

// Run initiates the process of retrieving URL information from the Wayback Machine API for a given domain.
//
// Parameters:
//   - domain (string): The target domain for which URLs are to be retrieved.
//   - cfg (*sources.Configuration): The configuration instance containing API keys,
//     the URL validation function, and any additional settings required by the source.
//
// Returns:
//   - (<-chan sources.Result): A channel that asynchronously emits sources.Result values.
//     Each result is either a discovered URL (ResultURL) or an error (ResultError)
//     encountered during the operation.
func (source *Source) Run(domain string, cfg *sources.Configuration) <-chan sources.Result {
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

			// Slicing as [1:] to skip first result by default
			for _, record := range getURLsResData[1:] {
				var URL string

				var valid bool

				if URL, valid = cfg.Validate(record[1]); !valid {
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

// Name returns the unique identifier for the data source.
// This identifier is used for logging, debugging, and associating results with the correct data source.
//
// Returns:
//   - name (string): The unique identifier for the data source.
func (source *Source) Name() (name string) {
	return sources.WAYBACK
}

// limiter is a rate limiter instance configured to control the number of requests
// sent to the Wayback Machine API. It ensures that no more than 40 requests are made per minute,
// with a minimum delay of 30 seconds between requests.
var limiter = hqgolimiter.New(&hqgolimiter.Configuration{
	RequestsPerMinute:     40,
	MinimumDelayInSeconds: 30,
})
