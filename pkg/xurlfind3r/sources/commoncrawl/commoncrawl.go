package commoncrawl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
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

		getIndexesRes, err := cfg.Client.Request().
			URL(getIndexesReqURL).
			Context(ctx).Do()
		if err != nil {
			result := sources.ErrorResult(source.Name(), fmt.Errorf("request failed: %w", err))

			results <- result

			return
		}

		if err = sources.CheckResponse(getIndexesRes.Std()); err != nil {
			_ = getIndexesRes.Body.Close()

			result := sources.ErrorResult(source.Name(), err)

			results <- result

			return
		}

		var getIndexesResData getIndexesResponse

		if err = json.NewDecoder(getIndexesRes.Body).Decode(&getIndexesResData); err != nil {
			result := sources.ErrorResult(source.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

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
			getPaginationRes, err := cfg.Client.Request().
				URL(CCIndexAPI).
				Param("url", "*."+domain).
				Param("output", "json").
				Param("fl", "url").
				Param("collapse", "urlkey").
				Param("showNumPages", "true").
				Context(ctx).Do()
			if err != nil {
				if getPaginationRes != nil {
					_ = getPaginationRes.Body.Close()
				}

				result := sources.ErrorResult(source.Name(), fmt.Errorf("request failed: %w", err))

				results <- result

				continue
			}

			if err = sources.CheckResponse(getPaginationRes.Std()); err != nil {
				_ = getPaginationRes.Body.Close()

				result := sources.ErrorResult(source.Name(), err)

				results <- result

				continue
			}

			var getPaginationData getPaginationResponse

			if err = json.NewDecoder(getPaginationRes.Body).Decode(&getPaginationData); err != nil {
				result := sources.ErrorResult(source.Name(), fmt.Errorf("failed to parse JSON response: %w", err))

				results <- result

				_ = getPaginationRes.Body.Close()

				continue
			}

			_ = getPaginationRes.Body.Close()

			if getPaginationData.Pages < 1 {
				continue
			}

			emptyPages := 0

			for page := range getPaginationData.Pages {
				if err := limiter.Wait(ctx); err != nil {
					result := sources.ErrorResult(source.Name(), fmt.Errorf("failed to wait on rate limiter: %w", err))

					results <- result

					break
				}

				var getURLsRes *hqgohttp.Response

				getURLsRes, err = cfg.Client.Request().
					URL(CCIndexAPI).
					Param("url", "*."+domain).
					Param("output", "json").
					Param("fl", "url").
					Param("collapse", "urlkey").
					Param("page", cast.ToString(page)).
					Context(ctx).Do()
				if err != nil {
					if getURLsRes != nil {
						_ = getURLsRes.Body.Close()
					}

					result := sources.ErrorResult(source.Name(), fmt.Errorf("request failed: %w", err))

					results <- result

					continue
				}

				if err = sources.CheckResponse(getURLsRes.Std()); err != nil {
					_ = getURLsRes.Body.Close()

					result := sources.ErrorResult(source.Name(), err)

					results <- result

					continue
				}

				scannedThisPage := 0

				done := make(chan struct{})

				go func() {
					defer close(done)

					scanner := bufio.NewScanner(getURLsRes.Body)

					scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

					for scanner.Scan() {
						scannedThisPage++

						var getURLsResData getURLsResponse

						if err = json.Unmarshal(scanner.Bytes(), &getURLsResData); err != nil {
							result := sources.ErrorResult(source.Name(), err)

							results <- result

							continue
						}

						if getURLsResData.Error != "" {
							result := sources.ErrorResult(source.Name(), fmt.Errorf("domain error: %s", getURLsResData.Error))

							results <- result

							continue
						}

						URL, valid := cfg.Validator(getURLsResData.URL)
						if !valid {
							continue
						}

						result := sources.URLResult(source.Name(), URL)

						results <- result
					}

					if scanErr := scanner.Err(); scanErr != nil && ctx.Err() == nil {
						result := sources.ErrorResult(source.Name(), fmt.Errorf("failed to read response body: %w", scanErr))

						results <- result
					}
				}()

				select {
				case <-done:
				case <-ctx.Done():
					_ = getURLsRes.Body.Close()

					<-done
				}

				if ctx.Err() != nil {
					_ = getURLsRes.Body.Close()

					return
				}

				if scannedThisPage == 0 {
					emptyPages++

					if emptyPages >= 3 {
						_ = getURLsRes.Body.Close()

						break
					}
				} else {
					emptyPages = 0
				}

				_ = getURLsRes.Body.Close()
			}
		}
	}()

	return results
}

const maxScanTokenSize = 1 << 20

var limiter = hqgolimiter.New(
	hqgolimiter.WithRequestsPerMinute(30),
)

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
