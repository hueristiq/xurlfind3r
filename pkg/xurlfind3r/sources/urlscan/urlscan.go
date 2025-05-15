// Package urlscan provides an implementation of the sources.Source interface
// for interacting with the urlscan.io API.
//
// The urlscan.io API enables scanning of URLs and retrieving associated data such as
// domain, MIME type, HTTP status, and more. This package defines a Source type that
// implements the Run and Name methods as specified by the sources.Source interface.
// The Run method queries the urlscan.io API for URLs associated with a target domain,
// handles pagination via the "search_after" parameter, validates discovered URLs using the
// provided configuration, and streams valid URLs or errors asynchronously via a channel.
package urlscan

import (
	"encoding/json"
	"errors"
	"strings"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/hq-go-http/header"
	"github.com/hueristiq/hq-go-http/mime"
	"github.com/hueristiq/hq-go-http/status"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
)

// searchResponse represents the structure of the JSON response returned by the urlscan.io API.
//
// It contains the following fields:
//   - Results: A slice of result objects, each containing details about a scanned page.
//     Each result includes a Page field with domain-related data and a Sort field used for pagination.
//   - Status: An integer representing the status code of the API response.
//   - Total: An integer representing the total number of results.
//   - Took: An integer representing the time taken for the search (in milliseconds).
//   - HasMore: A boolean indicating whether more results are available for pagination.
type searchResponse struct {
	Results []struct {
		Page struct {
			Domain   string `json:"domain"`
			MimeType string `json:"mimeType"`
			URL      string `json:"url"`
			Status   string `json:"status"`
		} `json:"page"`
		Sort []interface{} `json:"sort"`
	} `json:"results"`
	Status  int  `json:"status"`
	Total   int  `json:"total"`
	Took    int  `json:"took"`
	HasMore bool `json:"has_more"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the urlscan.io API.
type Source struct{}

// Run initiates the process of retrieving URL information from the urlscan.io API for a given domain.
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

		key, err := cfg.Keys.URLScan.PickRandom()
		if err != nil && !errors.Is(err, sources.ErrNoKeys) {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var after string

		for {
			searchReqURL := "https://urlscan.io/api/v1/search"
			searchReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"q":    "domain:" + domain,
					"size": "10000",
				},
				Headers: []hqgohttp.Header{
					hqgohttp.NewHeader(header.Accept.String(), mime.JSON.String(), hqgohttp.HeaderModeSet),
					hqgohttp.NewHeader("API-Key", key, hqgohttp.HeaderModeSet),
				},
			}

			if after != "" {
				searchReqCFG.Params["search_after"] = after
			}

			searchRes, err := hqgohttp.Get(searchReqURL, searchReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				break
			}

			var searchResData searchResponse

			if err = json.NewDecoder(searchRes.Body).Decode(&searchResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				searchRes.Body.Close()

				break
			}

			searchRes.Body.Close()

			if searchResData.Status == status.TooManyRequests.Int() {
				break
			}

			for _, result := range searchResData.Results {
				var URL string

				var valid bool

				if URL, valid = cfg.Validate(result.Page.URL); !valid {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}

			if !searchResData.HasMore {
				break
			}

			if len(searchResData.Results) < 1 {
				break
			}

			lastResult := searchResData.Results[len(searchResData.Results)-1]

			if lastResult.Sort != nil {
				var temp []string

				for index := range lastResult.Sort {
					temp = append(temp, cast.ToString(lastResult.Sort[index]))
				}

				after = strings.Join(temp, ",")
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
	return sources.URLSCAN
}
