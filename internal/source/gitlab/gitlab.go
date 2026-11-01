// Package gitlab implements GitLab blob search and raw-file retrieval.
package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
	"github.com/hueristiq/xurlfind3r/internal/source/internal/stream"
)

const maxPages = 100

const (
	// Snippet identifies text returned by the blob search itself.
	Snippet = "snippet"
	// Content identifies text retrieved from a repository file.
	Content = "content"
)

// Client is an immutable GitLab client safe for concurrent searches.
type Client struct {
	doer hqgosource.Doer
	key  string
}

// New constructs a client with an explicit private token.
func New(doer hqgosource.Doer, key string) (*Client, error) {
	if doer == nil {
		return nil, errors.New("gitlab: nil HTTP client")
	}

	if key == "" {
		return nil, errors.New("gitlab: empty private token")
	}

	return &Client{doer: doer, key: key}, nil
}

type searchItem struct {
	Data      string `json:"data"`
	ProjectID int    `json:"project_id"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
}

// Search streams blob snippets and raw file content, marked with Snippet or
// Content. At most 100 search pages are requested.
func (c *Client) Search(ctx context.Context, domain string) <-chan hqgosource.Result {
	results := make(chan hqgosource.Result, 1)

	if ctx == nil {
		results <- hqgosource.Result{Err: errors.New("gitlab: nil context")}

		close(results)

		return results
	}

	go func() {
		defer close(results)

		if strings.TrimSpace(domain) == "" {
			stream.Emit(ctx, results, hqgosource.Result{Err: errors.New("gitlab: empty domain")})

			return
		}

		stopped := false

		for page := 1; page <= maxPages; page++ {
			headers := http.Header{"Private-Token": {c.key}}
			query := url.Values{
				"scope":    {"blobs"},
				"search":   {domain},
				"per_page": {"100"},
				"page":     {strconv.Itoa(page)},
			}

			response, err := request.Do(ctx, c.doer, http.MethodGet, "https://gitlab.com/api/v4/search", http.NoBody, query, headers, c.key) //nolint:bodyclose // request.Decode closes the body.
			if err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			nextPageHeader := response.Header.Get("X-Next-Page")

			var items []searchItem
			if err := request.Decode(response, &items); err != nil {
				stream.Emit(ctx, results, hqgosource.Result{Err: err})

				return
			}

			for _, item := range items {
				if item.Data != "" && !stream.Emit(ctx, results, hqgosource.Result{Value: item.Data, Kind: Snippet}) {
					return
				}

				if !c.sendContent(ctx, item, results) {
					return
				}
			}

			if nextPageHeader == "" {
				stopped = true

				break
			}

			nextPage, err := strconv.Atoi(nextPageHeader)
			if err != nil || nextPage == 0 {
				stopped = true

				break
			}

			if nextPage == page {
				stream.Emit(ctx, results, hqgosource.Result{Warning: "pagination stopped: next page did not advance"})

				stopped = true

				break
			}

			page = nextPage - 1
		}

		if !stopped {
			stream.Emit(ctx, results, hqgosource.Result{Warning: fmt.Sprintf("pagination capped at %d pages", maxPages)})
		}
	}()

	return results
}

func (c *Client) sendContent(ctx context.Context, item searchItem, results chan<- hqgosource.Result) bool {
	headers := http.Header{"Private-Token": {c.key}}
	endpoint := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/files/%s/raw?%s",
		item.ProjectID, url.PathEscape(item.Path), url.Values{"ref": {item.Ref}}.Encode())

	response, err := request.Execute(ctx, c.doer, http.MethodGet, endpoint, http.NoBody, nil, headers, c.key)
	if err != nil {
		return stream.Emit(ctx, results, hqgosource.Result{Err: err})
	}

	if response.StatusCode != http.StatusOK {
		if response.Body != nil {
			_ = response.Body.Close()
		}

		return true
	}

	body, err := request.Read(response)
	if err != nil {
		return stream.Emit(ctx, results, hqgosource.Result{Err: err})
	}

	if len(body) == 0 {
		return true
	}

	return stream.Emit(ctx, results, hqgosource.Result{Value: string(body), Kind: Content})
}
