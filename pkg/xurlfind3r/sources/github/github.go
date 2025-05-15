// Package github provides an implementation of the sources.Source interface
// for interacting with the GitHub Code Search API.
//
// The GitHub API allows searching code repositories for occurrences of a given domain,
// which can reveal URLs or references associated with that domain. This package defines a
// Source type that implements the Run, Enumerate, and Name methods as specified by the
// sources.Source interface. The Run method initiates a GitHub code search query for a target
// domain, and the Enumerate method processes paginated search results, extracts URLs from both
// raw file content and text matches, and streams discovered URLs or errors via a channel.
package github

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/hq-go-http/header"
	"github.com/hueristiq/hq-go-http/status"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
)

// codeSearchResponse represents the structure of the JSON response returned by the GitHub code search API.
//
// It contains the total count of matching records and a slice of items where each item
// represents a code search result. Each item includes the repository file name, the HTML URL for the file,
// and any text matches found in the file.
type codeSearchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		TextMatches []struct {
			Fragment string `json:"fragment"`
		} `json:"text_matches"`
	} `json:"items"`
}

// Source represents the Common Crawl data source implementation.
// It implements the sources.Source interface, providing functionality
// for retrieving URLs by querying GitHub code search results.
type Source struct{}

// Run initiates the process of retrieving URL information from Github for a given domain.
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

		if len(cfg.Keys.Github) == 0 {
			return
		}

		tokens := NewTokenManager(cfg.Keys.Github)

		searchReqURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%q&sort=created&order=asc", domain)

		source.Enumerate(searchReqURL, tokens, cfg, results)
	}()

	return results
}

// Enumerate processes GitHub code search results by sending HTTP GET requests to the provided search URL,
// handling pagination via the Link header, and extracting URLs from raw file content and text matches.
//
// Parameters:
//   - searchReqURL (string): The URL for the GitHub code search API request.
//   - tokens (*Tokens): A token manager containing GitHub API tokens to handle rate limiting.
//   - cfg (*sources.Configuration): The configuration settings used for authentication and regex extraction.
//   - results (chan sources.Result): A channel to stream discovered URLs or errors.
func (source *Source) Enumerate(searchReqURL string, tokens *Tokens, cfg *sources.Configuration, results chan sources.Result) {
	token := tokens.Get()

	if token.RetryAfter > 0 {
		if len(tokens.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = tokens.Get()
		}
	}

	codeSearchResCFG := &hqgohttp.RequestConfiguration{
		Headers: []hqgohttp.Header{
			hqgohttp.NewHeader(header.Accept.String(), "application/vnd.github.v3.text-match+json", hqgohttp.HeaderModeSet),
			hqgohttp.NewHeader(header.Authorization.String(), "token "+token.Hash, hqgohttp.HeaderModeSet),
		},
	}

	codeSearchRes, err := hqgohttp.Get(searchReqURL, codeSearchResCFG)

	isForbidden := codeSearchRes != nil && codeSearchRes.StatusCode == status.Forbidden.Int()

	if err != nil && !isForbidden {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: source.Name(),
			Error:  err,
		}

		results <- result

		return
	}

	ratelimitRemaining := cast.ToInt64(codeSearchRes.Header.Get(header.XRatelimitRemaining.String()))
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(codeSearchRes.Header.Get(header.RetryAfter.String()))

		tokens.setCurrentTokenExceeded(retryAfterSeconds)

		source.Enumerate(searchReqURL, tokens, cfg, results)
	}

	var codeSearchResData codeSearchResponse

	if err = json.NewDecoder(codeSearchRes.Body).Decode(&codeSearchResData); err != nil {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: source.Name(),
			Error:  err,
		}

		results <- result

		codeSearchRes.Body.Close()

		return
	}

	codeSearchRes.Body.Close()

	for _, item := range codeSearchResData.Items {
		getRawContentReqURL := strings.ReplaceAll(item.HTMLURL, "https://github.com/", "https://raw.githubusercontent.com/")
		getRawContentReqURL = strings.ReplaceAll(getRawContentReqURL, "/blob/", "/")

		var getRawContentRes *http.Response

		getRawContentRes, err = hqgohttp.Get(getRawContentReqURL)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			continue
		}

		if getRawContentRes.StatusCode != status.OK.Int() {
			getRawContentRes.Body.Close()

			continue
		}

		scanner := bufio.NewScanner(getRawContentRes.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			URLs := cfg.Extractor.FindAllString(line, -1)

			for _, URL := range URLs {
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

		if err = scanner.Err(); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getRawContentRes.Body.Close()

			continue
		}

		getRawContentRes.Body.Close()

		for _, textMatch := range item.TextMatches {
			URLs := cfg.Extractor.FindAllString(textMatch.Fragment, -1)

			for _, URL := range URLs {
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

	links := header.ParseLinkHeader(codeSearchRes.Header.Get(header.Link.String()))

	for _, link := range links {
		if link.Rel == "next" {
			nextURL, err := url.QueryUnescape(link.URL)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			source.Enumerate(nextURL, tokens, cfg, results)
		}
	}
}

// Name returns the unique identifier for the data source.
// This identifier is used for logging, debugging, and associating results with the correct data source.
//
// Returns:
//   - name (string): The unique identifier for the data source.
func (source *Source) Name() (name string) {
	return sources.GITHUB
}
