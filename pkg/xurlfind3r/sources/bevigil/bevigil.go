// Package bevigil provides an implementation of the sources.Source interface
// for interacting with the Bevigil data source.
//
// The Bevigil service exposes an API endpoint that returns URLs associated with a given domain.
// This package defines a Source type that implements the Run and Name methods as specified
// by the sources.Source interface. The Run method retrieves URLs for a specified domain,
// decodes the JSON response, and streams each valid URL asynchronously via a channel.
package bevigil

import (
	"encoding/json"
	"fmt"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

// getURLsResponse defines the structure for decoding the JSON response from the Bevigil API.
//
// Fields:
//   - Domain (string): The queried domain as returned by the API.
//   - URLs ([]string): A slice of URL strings associated with the domain.
type getURLsResponse struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

// Source represents the Bevigil data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the Bevigil OSINT API.
type Source struct{}

// Run initiates the URL discovery process for the specified domain using the Bevigil API.
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

		key, err := cfg.Keys.Bevigil.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		getURLsReqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", domain)
		getURLsReqCFG := &hqgohttp.RequestConfiguration{
			Headers: []hqgohttp.Header{
				hqgohttp.NewHeader("X-Access-Token", key, hqgohttp.HeaderModeSet),
			},
		}

		getURLsRes, err := hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getURLsResData getURLsResponse

		if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getURLsRes.Body.Close()

			return
		}

		getURLsRes.Body.Close()

		for _, URL := range getURLsResData.URLs {
			var valid bool

			if URL, valid = cfg.Validate(URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: source.Name(),
				Value:  URL,
			}

			results <- result
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
	return sources.BEVIGIL
}
