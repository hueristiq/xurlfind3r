package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (s *Source) NeedsKeys() (needs bool) {
	needs = true

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := s.keys.PickRandom()
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to select key: %w", err))

			results <- result

			return
		}

		page := 1

		for page > 0 {
			if page > sources.MaxPages {
				result := sources.WarningResult(s.Name(), fmt.Sprintf("pagination capped at %d pages", sources.MaxPages))

				results <- result

				break
			}

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
				if searchRes != nil {
					_ = searchRes.Body.Close()
				}

				result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

				results <- result

				return
			}

			if err = sources.CheckResponse(searchRes, key); err != nil {
				_ = searchRes.Body.Close()

				result := sources.ErrorResult(s.Name(), err)

				results <- result

				return
			}

			nextPage := searchRes.Header.Get("X-Next-Page")

			var items []searchResponseItem

			if err = json.NewDecoder(searchRes.Body).Decode(&items); err != nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

				results <- result

				_ = searchRes.Body.Close()

				return
			}

			_ = searchRes.Body.Close()

			for _, item := range items {
				sources.SendURLs(results, cfg, s.Name(), sources.ExtractURLsAll(cfg.Extractor, item.Data))

				getRawContentReqURL := fmt.Sprintf(
					"https://gitlab.com/api/v4/projects/%d/repository/files/%s/raw?%s",
					item.ProjectID,
					url.PathEscape(item.Path),
					url.Values{"ref": {item.Ref}}.Encode(),
				)

				getRawContentRes, err := cfg.Client.NewRequest().
					URL(getRawContentReqURL).
					SetHeader("PRIVATE-TOKEN", key).
					Do(ctx)
				if err != nil {
					if getRawContentRes != nil {
						_ = getRawContentRes.Body.Close()
					}

					result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

					results <- result

					continue
				}

				if getRawContentRes.StatusCode != hqgohttpstatus.OK.Int() {
					_ = getRawContentRes.Body.Close()

					continue
				}

				rawContent, err := io.ReadAll(io.LimitReader(getRawContentRes.Body, maxScanBytes))
				if err != nil {
					result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to read response body: %w", err))

					results <- result

					_ = getRawContentRes.Body.Close()

					return
				}

				_ = getRawContentRes.Body.Close()

				content := string(rawContent)

				sources.SendURLs(results, cfg, s.Name(), sources.ExtractURLsAll(cfg.Extractor, content))
				sources.SendURLs(results, cfg, s.Name(), sources.ExtractCollapsedURLs(cfg.Extractor, content))
			}

			nextPageNumber := cast.ToInt(nextPage)

			if nextPageNumber == page {
				result := sources.WarningResult(s.Name(), "pagination stopped: next page did not advance")

				results <- result

				break
			}

			page = nextPageNumber
		}
	}()

	return results
}

const maxScanBytes = 8 << 20

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
