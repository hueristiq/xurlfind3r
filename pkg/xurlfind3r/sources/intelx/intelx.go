// Package intelx provides an implementation of the sources.Source interface
// for interacting with the IntelX API.
//
// The IntelX API offers phonebook search capabilities that can be used to retrieve
// URLs related to a target domain. This package defines a Source type that implements
// the Run and Name methods as specified by the sources.Source interface. The Run method
// initiates a phonebook search request, polls for detailed results, and streams discovered
// URLs or errors via a channel.
package intelx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/hq-go-http/header"
	"github.com/hueristiq/hq-go-http/mime"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

// searchRequestBody represents the structure of the JSON request body sent to the IntelX API
// to initiate a phonebook search.
//
// Fields:
//   - Term (string): The search term, in this case the target domain.
//   - MaxResults (int): The maximum number of results to retrieve.
//   - Media (int): Media type filter (0 indicates no filtering).
//   - Target (int): The target type for the search (1 = Domains, 2 = Emails, 3 = URLs).
//   - Timeout (time.Duration): The maximum duration allowed for the search query.
type searchRequestBody struct {
	Term       string        `json:"term"`
	MaxResults int           `json:"maxresults"`
	Media      int           `json:"media"`
	Target     int           `json:"target"`
	Timeout    time.Duration `json:"timeout"`
}

// searchResponse represents the structure of the JSON response returned by the IntelX API
// after initiating a phonebook search.
//
// Fields:
//   - ID (string): A unique identifier for the search query.
//   - SelfSelectWarning (bool): Indicates if a self-selection warning was returned.
//   - Status (int): The status code of the search query.
//   - AltTerm (string): An alternative search term, if provided.
//   - AltTermH (string): A hashed version of the alternative search term.
type searchResponse struct {
	ID                string `json:"id"`
	SelfSelectWarning bool   `json:"selfselectwarning"`
	Status            int    `json:"status"`
	AltTerm           string `json:"altterm"`
	AltTermH          string `json:"alttermh"`
}

// getResultsResponse represents the structure of the JSON response returned by the IntelX API
// when polling for detailed search results.
//
// Fields:
//   - Selectors ([]struct): A slice of objects where each object contains a subdomain value
//     under "selectorvalue".
//   - Status (int): The status of the results retrieval. A value of 0 or 3 indicates that results
//     are still being processed.
type getResultsResponse struct {
	Selectors []struct {
		Selectvalue string `json:"selectorvalue"`
	} `json:"selectors"`
	Status int `json:"status"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the IntelX API.
type Source struct{}

// Run initiates the process of retrieving URL information from IntelX for a given domain.
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

		key, err := cfg.Keys.IntelX.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			return
		}

		intelXHost := parts[0]
		intelXKey := parts[1]

		if intelXKey == "" || intelXHost == "" {
			return
		}

		searchReqURL := fmt.Sprintf("https://%s/phonebook/search?", intelXHost)
		searchReqBody := searchRequestBody{
			Term:       "*" + domain,
			MaxResults: 100000,
			Media:      0,
			Target:     3, // 1 = Domains | 2 = Emails | 3 = URLs
			Timeout:    20,
		}

		var searchReqBodyBytes []byte

		searchReqBodyBytes, err = json.Marshal(searchReqBody)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		searchReqBodyReader := bytes.NewBuffer(searchReqBodyBytes)

		searchReqCFG := &hqgohttp.RequestConfiguration{
			Params: map[string]string{
				"k": intelXKey,
			},
			Headers: map[string]string{
				header.ContentType.String(): mime.JSON.String(),
			},
		}

		var searchRes *http.Response

		searchRes, err = hqgohttp.Post(searchReqURL, searchReqBodyReader, searchReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
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

			return
		}

		searchRes.Body.Close()

		getResultsReqURL := fmt.Sprintf("https://%s/phonebook/search/result", intelXHost)
		getResultsReqCFG := &hqgohttp.RequestConfiguration{
			Params: map[string]string{
				"k":     intelXKey,
				"id":    searchResData.ID,
				"limit": "10000",
			},
		}
		status := 0

		for status == 0 || status == 3 {
			var getResultsRes *http.Response

			getResultsRes, err = hqgohttp.Get(getResultsReqURL, getResultsReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			var getResultsResData getResultsResponse

			if err = json.NewDecoder(getResultsRes.Body).Decode(&getResultsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				getResultsRes.Body.Close()

				return
			}

			getResultsRes.Body.Close()

			status = getResultsResData.Status

			for _, hostname := range getResultsResData.Selectors {
				var URL string

				var valid bool

				if URL, valid = cfg.Validate(hostname.Selectvalue); !valid {
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
	return sources.INTELLIGENCEX
}
