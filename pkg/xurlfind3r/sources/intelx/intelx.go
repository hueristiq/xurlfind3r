package intelx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgourl "github.com/hueristiq/hq-go-url"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type searchRequest struct {
	Term       string        `json:"term"`
	Timeout    time.Duration `json:"timeout"`
	Target     int           `json:"target"`
	MaxResults int           `json:"maxresults"`
	Media      int           `json:"media"`
}
type searchResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type getResultsResponse struct {
	Selectors []struct {
		Selectvalue string `json:"selectorvalue"`
	} `json:"selectors"`
	Status int `json:"status"`
}

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
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

		searchReqURL := fmt.Sprintf("https://%s/phonebook/search?k=%s", intelXHost, intelXKey)
		searchReqBody := searchRequest{
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

		var searchRes *http.Response

		searchRes, err = hqgohttp.POST(searchReqURL).AddHeader("Content-Type", "application/json").Body(searchReqBodyReader).Send()
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

		getResultsReqURL := fmt.Sprintf("https://%s/phonebook/search/result?k=%s&id=%s&limit=10000", intelXHost, intelXKey, searchResData.ID)
		status := 0

		for status == 0 || status == 3 {
			var getResultsRes *http.Response

			getResultsRes, err = hqgohttp.GET(getResultsReqURL).Send()
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
				URL := hostname.Selectvalue
				URL = sources.FixURL(URL)

				parsedURL, err := up.Parse(URL)
				if err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					return
				}

				parsedURL.Path = strings.Split(parsedURL.Path, ":")[0]

				URL = parsedURL.String()

				if !cfg.IsInScope(URL) {
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

func (source *Source) Name() string {
	return sources.INTELLIGENCEX
}

var up = hqgourl.NewParser(hqgourl.ParserWithDefaultScheme("http"))
