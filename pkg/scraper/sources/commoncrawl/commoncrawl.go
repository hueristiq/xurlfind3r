package commoncrawl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
	"github.com/valyala/fasthttp"
)

type getIndexesResponse []struct {
	ID  string `json:"id"`
	API string `json:"cdx-API"`
}

type getURLsResponse struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	if config.IncludeSubdomains {
		domain = "*." + domain
	}

	go func() {
		defer close(results)

		getIndexesReqURL := "https://index.commoncrawl.org/collinfo.json"

		var err error

		var getIndexesRes *fasthttp.Response

		getIndexesRes, err = httpclient.SimpleGet(getIndexesReqURL)
		if err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getIndexesResData getIndexesResponse

		err = json.Unmarshal(getIndexesRes.Body(), &getIndexesResData)
		if err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		wg := new(sync.WaitGroup)

		for _, indexData := range getIndexesResData {
			wg.Add(1)

			go func(API string) {
				defer wg.Done()

				getURLsReqHeaders := map[string]string{
					"Host": "index.commoncrawl.org",
				}

				getURLsReqURL := fmt.Sprintf("%s?url=%s/*&output=json&fl=url", API, domain)

				var err error

				var getURLsRes *fasthttp.Response

				getURLsRes, err = httpclient.Get(getURLsReqURL, "", getURLsReqHeaders)
				if err != nil {
					result := sources.Result{
						Type:   sources.Error,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					return
				}

				scanner := bufio.NewScanner(bytes.NewReader(getURLsRes.Body()))

				for scanner.Scan() {
					var getURLsResData getURLsResponse

					err = json.Unmarshal(scanner.Bytes(), &getURLsResData)
					if err != nil {
						result := sources.Result{
							Type:   sources.Error,
							Source: source.Name(),
							Error:  err,
						}

						results <- result

						return
					}

					if getURLsResData.Error != "" {
						return
					}

					URL := getURLsResData.URL

					if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
						continue
					}

					result := sources.Result{
						Type:   sources.URL,
						Source: source.Name(),
						Value:  URL,
					}

					results <- result
				}

				if err = scanner.Err(); err != nil {
					result := sources.Result{
						Type:   sources.Error,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					return
				}
			}(indexData.API)
		}

		wg.Wait()
	}()

	return results
}

func (source *Source) Name() string {
	return "commoncrawl"
}
