package gitlab

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	hqgohttpstatus "github.com/hueristiq/hq-lib-http-go/status"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type searchResponseItem struct {
	Data      string `json:"data"`
	ProjectID int    `json:"project_id"`
	Path      string `json:"path"`
	Ref       string `json:"ref"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.Gitlab

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := s.keys.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to select key: %w", err),
			}

			results <- result

			return
		}

		page := 1

		for page > 0 {
			searchReqURL := "https://gitlab.com/api/v4/search"

			searchRes, err := cfg.Client.NewRequest().
				URL(searchReqURL).
				Param("scope", "blobs").
				Param("search", domain).
				Param("per_page", "100").
				Param("page", cast.ToString(page)).
				SetHeader("PRIVATE-TOKEN", key).
				Do(ctx)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				return
			}

			nextPage := searchRes.Header.Get("X-Next-Page")

			var items []searchResponseItem

			if err = json.NewDecoder(searchRes.Body).Decode(&items); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				_ = searchRes.Body.Close()

				return
			}

			_ = searchRes.Body.Close()

			for _, item := range items {
				URLs := sources.ExtractURLs(cfg.Extractor, item.Data)

				for _, URL := range URLs {
					var valid bool

					if URL, valid = cfg.Validator(URL); !valid {
						continue
					}

					result := sources.Result{
						Type:   sources.ResultURL,
						Source: s.Name(),
						Value:  URL,
					}

					results <- result
				}

				getRawContentReqURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/files/%s/raw?ref=%s", item.ProjectID, url.PathEscape(item.Path), url.QueryEscape(item.Ref))

				getRawContentRes, err := cfg.Client.NewRequest().
					URL(getRawContentReqURL).
					SetHeader("PRIVATE-TOKEN", key).
					Do(ctx)
				if err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: s.Name(),
						Error:  fmt.Errorf("request failed: %w", err),
					}

					results <- result

					continue
				}

				if getRawContentRes.StatusCode != hqgohttpstatus.OK.Int() {
					_ = getRawContentRes.Body.Close()

					continue
				}

				scanner := bufio.NewScanner(getRawContentRes.Body)

				for scanner.Scan() {
					line := scanner.Text()

					if line == "" {
						continue
					}

					URLs := sources.ExtractURLs(cfg.Extractor, line)

					for _, URL := range URLs {
						var valid bool

						if URL, valid = cfg.Validator(URL); !valid {
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

				if err = scanner.Err(); err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: s.Name(),
						Error:  fmt.Errorf("failed to read response body: %w", err),
					}

					results <- result

					_ = getRawContentRes.Body.Close()

					return
				}

				_ = getRawContentRes.Body.Close()
			}

			page = cast.ToInt(nextPage)
		}
	}()

	return results
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
