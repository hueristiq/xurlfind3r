// Package commoncrawl provides an implementation of the sources.Source interface
// for interacting with the Common Crawl index.
//
// The Common Crawl index offers archived web data that can be leveraged to discover
// subdomains or URLs for a given domain by searching historical records. This package
// defines a Source type that implements the Run and Name methods as specified by the
// sources.Source interface. The Run method retrieves index metadata, selects relevant
// indexes based on recent years, queries each index for URL records matching the target
// domain, validates the returned URLs using a provided function, and streams valid URLs
// or errors via a channel.
package commoncrawl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
	hqgohttp "go.source.hueristiq.com/http"
	"go.source.hueristiq.com/http/header"
)

// getIndexesResponse represents the structure of the JSON response returned by
// the Common Crawl index metadata endpoint.
//
// It is defined as a slice of anonymous structs, where each struct contains:
//   - ID: A string identifier for the index.
//   - Name: The name of the index.
//   - TimeGate: A URL for time-based redirection.
//   - CDXAPI: A string containing the API endpoint URL for that index.
//   - From: A string representing the start date of the index.
//   - To: A string representing the end date of the index.
type getIndexesResponse []struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TimeGate string `json:"timegate"`
	CDXAPI   string `json:"cdx-api"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// getPaginationResponse represents the structure of the JSON response that provides
// pagination information for a Common Crawl index query.
//
// It contains the following fields:
//   - Blocks: The number of data blocks available.
//   - PageSize: The number of records per page.
//   - Pages: The total number of pages available for the query.
type getPaginationResponse struct {
	Blocks   uint `json:"blocks"`
	PageSize uint `json:"pageSize"`
	Pages    uint `json:"pages"`
}

// getURLsResponse represents the structure of each JSON record returned when querying
// a Common Crawl index for URLs.
//
// It contains the following fields:
//   - URL: A string representing a discovered URL.
//   - Error: A string describing an error encountered for the record, if any.
type getURLsResponse struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs from the Common Crawl index.
type Source struct{}

// Run initiates the process of retrieving URL information from the Common Crawl index
// for a given domain.
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

		getIndexesReqURL := "https://index.commoncrawl.org/collinfo.json"

		getIndexesRes, err := hqgohttp.Get(getIndexesReqURL)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getIndexesResData getIndexesResponse

		if err = json.NewDecoder(getIndexesRes.Body).Decode(&getIndexesResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getIndexesRes.Body.Close()

			return
		}

		getIndexesRes.Body.Close()

		year := time.Now().Year()
		years := make([]string, 0)
		maxYearsBack := 5

		for i := range maxYearsBack {
			years = append(years, strconv.Itoa(year-i))
		}

		searchIndexes := make(map[string]string)

		for _, year := range years {
			for _, CCIndex := range getIndexesResData {
				if strings.Contains(CCIndex.ID, year) {
					if _, ok := searchIndexes[year]; !ok {
						searchIndexes[year] = CCIndex.CDXAPI

						break
					}
				}
			}
		}

		for _, CCIndexAPI := range searchIndexes {
			getPaginationReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"url":          "*." + domain,
					"output":       "json",
					"fl":           "url",
					"showNumPages": "true",
				},
				Headers: map[string]string{
					header.Host.String(): "index.commoncrawl.org",
				},
			}

			getPaginationRes, err := hqgohttp.Get(CCIndexAPI, getPaginationReqCFG)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				continue
			}

			var getPaginationData getPaginationResponse

			if err = json.NewDecoder(getPaginationRes.Body).Decode(&getPaginationData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				getPaginationRes.Body.Close()

				continue
			}

			getPaginationRes.Body.Close()

			if getPaginationData.Pages < 1 {
				continue
			}

			for page := range getPaginationData.Pages {
				var getURLsRes *http.Response

				getURLsReqCFG := &hqgohttp.RequestConfiguration{
					Params: map[string]string{
						"url":    "*." + domain,
						"output": "json",
						"fl":     "url",
						"page":   cast.ToString(page),
					},
					Headers: map[string]string{
						header.Host.String(): "index.commoncrawl.org",
					},
				}

				getURLsRes, err = hqgohttp.Get(CCIndexAPI, getURLsReqCFG)
				if err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					continue
				}

				scanner := bufio.NewScanner(getURLsRes.Body)

				for scanner.Scan() {
					var getURLsResData getURLsResponse

					if err = json.Unmarshal(scanner.Bytes(), &getURLsResData); err != nil {
						result := sources.Result{
							Type:   sources.ResultError,
							Source: source.Name(),
							Error:  err,
						}

						results <- result

						continue
					}

					if getURLsResData.Error != "" {
						result := sources.Result{
							Type:   sources.ResultError,
							Source: source.Name(),
							Error:  fmt.Errorf("%w: %s", errStatic, getURLsResData.Error),
						}

						results <- result

						continue
					}

					var URL string

					var valid bool

					if URL, valid = cfg.Validate(getURLsResData.URL); !valid {
						continue
					}

					result := sources.Result{
						Type:   sources.ResultURL,
						Source: source.Name(),
						Value:  URL,
					}

					results <- result
				}

				if err = scanner.Err(); err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					getURLsRes.Body.Close()

					continue
				}

				getURLsRes.Body.Close()
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
	return sources.COMMONCRAWL
}

// errStatic is a sentinel error used to prepend error messages when a
// record-specific error is encountered in the Common Crawl responses.
var errStatic = errors.New("something went wrong")
