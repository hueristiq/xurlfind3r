package intelx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgohttpheader "github.com/hueristiq/hq-go-http/header"
	hqgohttpmime "github.com/hueristiq/hq-go-http/mime"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type searchRequestBody struct {
	Term       string        `json:"term"`
	MaxResults int           `json:"maxresults"`
	Media      int           `json:"media"`
	Target     int           `json:"target"`
	Timeout    time.Duration `json:"timeout"`
}

type searchResponse struct {
	ID                string `json:"id"`
	SelfSelectWarning bool   `json:"selfselectwarning"`
	Status            int    `json:"status"`
	AltTerm           string `json:"altterm"`
	AltTermH          string `json:"alttermh"`
}

type getResultsResponse struct {
	Selectors []struct {
		Selectvalue string `json:"selectorvalue"`
	} `json:"selectors"`
	Status int `json:"status"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.INTELLIGENCEX

	return
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := s.keys.PickRandom()
		if key == "" || err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to select key: %w", err),
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
				Source: s.Name(),
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
			Headers: []hqgohttp.Header{
				hqgohttp.NewSetHeader(hqgohttpheader.ContentType.String(), hqgohttpmime.JSON.String()),
			},
		}

		var searchRes *http.Response

		searchRes, err = hqgohttp.Post(searchReqURL, searchReqBodyReader, searchReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("request failed: %w", err),
			}

			results <- result

			return
		}

		var searchResData searchResponse

		if err = json.NewDecoder(searchRes.Body).Decode(&searchResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to parse JSON response: %w", err),
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
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", err),
				}

				results <- result

				return
			}

			var getResultsResData getResultsResponse

			if err = json.NewDecoder(getResultsRes.Body).Decode(&getResultsResData); err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("failed to parse JSON response: %w", err),
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

				if URL, valid = cfg.Validator(hostname.Selectvalue); !valid {
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
