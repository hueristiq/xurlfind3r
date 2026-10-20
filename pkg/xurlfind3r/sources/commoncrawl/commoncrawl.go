package commoncrawl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	hqgolimiter "github.com/hueristiq/hq-lib-limiter-go"
	"github.com/spf13/cast"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getIndexesResponse []struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TimeGate string `json:"timegate"`
	CDXAPI   string `json:"cdx-api"`
	From     string `json:"from"`
	To       string `json:"to"`
}

type getPaginationResponse struct {
	Blocks   uint `json:"blocks"`
	PageSize uint `json:"pageSize"`
	Pages    uint `json:"pages"`
}

type getURLsResponse struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Commoncrawl

	return
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

func (source *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		getIndexesReqURL := "https://index.commoncrawl.org/collinfo.json"

		getIndexesRes, err := cfg.Client.NewRequest().
			URL(getIndexesReqURL).
			Do(ctx)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  fmt.Errorf("request failed: %w", err),
			}

			results <- result

			return
		}

		var getIndexesResData getIndexesResponse

		if err = json.NewDecoder(getIndexesRes.Body).Decode(&getIndexesResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  fmt.Errorf("failed to parse JSON response: %w", err),
			}

			results <- result

			_ = getIndexesRes.Body.Close()

			return
		}

		_ = getIndexesRes.Body.Close()

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
			getPaginationRes, err := cfg.Client.NewRequest().
				URL(CCIndexAPI).
				Param("url", "*."+domain).
				Param("output", "json").
				Param("fl", "url").
				Param("collapse", "urlkey").
				Param("showNumPages", "true").
				Do(ctx)
			if err != nil {
				if getPaginationRes != nil {
					_ = getPaginationRes.Body.Close()
				}

				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				continue
			}

			var getPaginationData getPaginationResponse

			if err = json.NewDecoder(getPaginationRes.Body).Decode(&getPaginationData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
				}

				results <- result

				_ = getPaginationRes.Body.Close()

				continue
			}

			_ = getPaginationRes.Body.Close()

			if getPaginationData.Pages < 1 {
				continue
			}

			for page := range getPaginationData.Pages {
				if err := limiter.Wait(ctx); err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  fmt.Errorf("failed to wait on rate limiter: %w", err),
					}

					results <- result

					break
				}

				var getURLsRes *http.Response

				getURLsRes, err = cfg.Client.NewRequest().
					URL(CCIndexAPI).
					Param("url", "*."+domain).
					Param("output", "json").
					Param("fl", "url").
					Param("collapse", "urlkey").
					Param("page", cast.ToString(page)).
					Do(ctx)
				if err != nil {
					if getURLsRes != nil {
						_ = getURLsRes.Body.Close()
					}

					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  fmt.Errorf("request failed: %w", err),
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
							Error:  fmt.Errorf("domain error: %s", getURLsResData.Error),
						}

						results <- result

						continue
					}

					var URL string

					var valid bool

					if URL, valid = cfg.Validator(getURLsResData.URL); !valid {
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
						Error:  fmt.Errorf("failed to read response body: %w", err),
					}

					results <- result

					_ = getURLsRes.Body.Close()

					continue
				}

				_ = getURLsRes.Body.Close()
			}
		}
	}()

	return results
}

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(30),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
