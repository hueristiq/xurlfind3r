// Package otx provides an implementation of the sources.Source interface
// for interacting with the Open Threat Exchange (OTX) API.
//
// The OTX API offers threat intelligence data, including URLs associated with a given domain.
// This package defines a Source type that implements the Run and Name methods as specified by the
// sources.Source interface. The Run method retrieves URL information for a target domain by
// paginating through the OTX API's URL list endpoint, validating each discovered URL using the provided
// configuration, and streaming valid URLs or errors via a channel.
package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
	hqgohttp "go.source.hueristiq.com/http"
)

// getURLsResponse represents the structure of the JSON response returned by the
// OTX API when querying for URL information associated with a target domain.
//
// It contains the following fields:
//   - URLList ([]struct): A slice of objects where each object represents a URL record.
//     Each URL record includes:
//   - URL (string): The discovered URL.
//   - Domain (string): The domain associated with the URL.
//   - Hostname (string): The hostname extracted from the URL.
//   - Result (struct): A nested object containing additional details, including:
//   - URLWorker (struct): Contains the IP address (IP) and HTTP response code (HTTPCode)
//     from the worker that processed the URL.
//   - HTTPCode (int): The HTTP response code associated with the URL.
//   - Encoded (string): An encoded version of the URL.
//   - PageNum (int): The current page number of the paginated response.
//   - Limit (int): The maximum number of records returned per page.
//   - Paged (bool): Indicates whether the response is paginated.
//   - HasNext (bool): Indicates if there are additional pages of results available.
//   - FullSize (int): The total number of URL records available.
//   - ActualSize (int): The actual number of URL records returned in the current response.
type getURLsResponse struct {
	URLList []struct {
		URL      string `json:"url"`
		Domain   string `json:"domain"`
		Hostname string `json:"hostname"`
		Result   struct {
			URLWorker struct {
				IP       string `json:"ip"`
				HTTPCode int    `json:"http_code"`
			} `json:"urlworker"`
		} `json:"result"`
		HTTPCode int    `json:"httpcode"`
		Encoded  string `json:"encoded"`
	} `json:"url_list"`
	PageNum    int  `json:"page_num"`
	Limit      int  `json:"limit"`
	Paged      bool `json:"paged"`
	HasNext    bool `json:"has_next"`
	FullSize   int  `json:"full_size"`
	ActualSize int  `json:"actual_size"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the Open Threat Exchange API.
type Source struct{}

// Run initiates the process of retrieving URL information from Open Threat Exchange for a given domain.
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

		for page := 1; ; page++ {
			getURLsReqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list", domain)
			getURLsReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"limit": "100",
					"page":  cast.ToString(page),
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

			for _, item := range getURLsResData.URLList {
				var URL string

				var valid bool

				if URL, valid = cfg.Validate(item.URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}

			if !getURLsResData.HasNext {
				break
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
	return sources.OPENTHREATEXCHANGE
}
