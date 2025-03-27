// Package virustotal provides an implementation of the sources.Source interface
// for interacting with the VirusTotal API.
//
// The VirusTotal API aggregates threat intelligence data for domains, URLs, and files.
// This package defines a Source type that implements the Run and Name methods as specified
// by the sources.Source interface. The Run method retrieves URL information for a target domain
// by querying the VirusTotal domain report endpoint, processes the detected URLs, subdomains,
// and undetected URLs from the response, validates them using the provided configuration, and
// streams valid URLs or errors asynchronously via a channel.
package virustotal

import (
	"encoding/json"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	hqgohttp "go.source.hueristiq.com/http"
	hqgolimiter "go.source.hueristiq.com/limiter"
)

// getDomainReportResponse represents the structure of the JSON response returned by the VirusTotal
// API when requesting a domain report.
//
// It contains the following fields:
//   - DetectedURLs ([]struct): A slice of objects, each containing a detected URL from the domain report.
//     Each object includes:
//   - URL (string): The URL that was detected.
//   - Subdomains ([]string): A slice of subdomains discovered in the domain report.
//   - UndetectedURLs ([][]interface{}): A slice of arrays where each array represents an undetected URL.
//     The first element of each array is expected to be a string URL.
type getDomainReportResponse struct {
	DetectedURLs []struct {
		URL string `json:"url"`
	} `json:"detected_urls"`
	Subdomains     []string        `json:"subdomains"`
	UndetectedURLs [][]interface{} `json:"undetected_urls"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the VirusTotal API.
type Source struct{}

// Run initiates the process of retrieving URL information from the VirusTotal API for a given domain.
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

		key, err := cfg.Keys.VirusTotal.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		getDomainReportReqURL := "https://www.virustotal.com/vtapi/v2/domain/report"
		getDomainReportReqCFG := &hqgohttp.RequestConfiguration{
			Params: map[string]string{
				"apikey": key,
				"domain": domain,
			},
		}

		limiter.Wait()

		getDomainReportRes, err := hqgohttp.Get(getDomainReportReqURL, getDomainReportReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getDomainReportResData getDomainReportResponse

		if err = json.NewDecoder(getDomainReportRes.Body).Decode(&getDomainReportResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getDomainReportRes.Body.Close()

			return
		}

		getDomainReportRes.Body.Close()

		for _, detectedURL := range getDomainReportResData.DetectedURLs {
			var URL string

			var valid bool

			if URL, valid = cfg.Validate(detectedURL.URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: source.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, subdomain := range getDomainReportResData.Subdomains {
			var URL string

			var valid bool

			if URL, valid = cfg.Validate(subdomain); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: source.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, undetectedURL := range getDomainReportResData.UndetectedURLs {
			if len(undetectedURL) > 0 {
				if URL, ok := undetectedURL[0].(string); ok {
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
	return sources.VIRUSTOTAL
}

// limiter is a rate limiter instance configured to control the number of requests
// sent to the VirusTotal API. It ensures that no more than 4 requests are made per minute,
// with a minimum delay of 30 seconds between requests.
var limiter = hqgolimiter.New(&hqgolimiter.Configuration{
	RequestsPerMinute:     4,
	MinimumDelayInSeconds: 30,
})
