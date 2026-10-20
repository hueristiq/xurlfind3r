package intelx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	hqgohttpheader "github.com/hueristiq/hq-lib-http-go/header"
	hqgohttpmethod "github.com/hueristiq/hq-lib-http-go/method"
	hqgohttpmime "github.com/hueristiq/hq-lib-http-go/mime"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type searchRequestBody struct {
	Term       string `json:"term"`
	MaxResults int    `json:"maxresults"`
	Media      int    `json:"media"`
	Target     int    `json:"target"`
	Timeout    int    `json:"timeout"`
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
	keys            sources.Keys
	pollInterval    time.Duration
	pollAttemptsMax int
}

func (s *Source) Name() (name string) {
	name = sources.IntelligenceX

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
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to select key: %w", err),
			}

			results <- result

			return
		}

		if key == "" {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  errors.New("failed to select key: selected key is empty"),
			}

			results <- result

			return
		}

		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  errors.New("failed to parse key: expected host:key format"),
			}

			results <- result

			return
		}

		intelXHost := parts[0]
		intelXKey := parts[1]

		if intelXKey == "" || intelXHost == "" {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  errors.New("failed to parse key: host or key is empty"),
			}

			results <- result

			return
		}

		searchReqURL := "https://" + intelXHost + "/phonebook/search"
		searchReqBody := searchRequestBody{
			Term:       "*" + domain,
			MaxResults: 100000,
			Media:      0,
			Target:     3, // 1 = Domains | 2 = Emails | 3 = URLs
			Timeout:    defaultSearchTimeout,
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

		var searchRes *http.Response

		searchRes, err = cfg.Client.NewRequest().
			Method(hqgohttpmethod.POST).
			URL(searchReqURL).
			Param("k", intelXKey).
			SetHeader(hqgohttpheader.ContentType.String(), hqgohttpmime.JSON.String()).
			Body(searchReqBodyReader).
			Do(ctx)
		if err != nil {
			if searchRes != nil {
				_ = searchRes.Body.Close()
			}

			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("request failed: %w", sources.RedactError(err, intelXKey)),
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

			_ = searchRes.Body.Close()

			return
		}

		_ = searchRes.Body.Close()

		getResultsReqURL := "https://" + intelXHost + "/phonebook/search/result"
		status := 0

		for attempts := 0; status == 0 || status == 3; attempts++ {
			if attempts >= s.pollAttemptsMax {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("giving up after %d result polls", attempts),
				}

				results <- result

				return
			}

			if attempts > 0 {
				timer := time.NewTimer(s.pollInterval)

				select {
				case <-ctx.Done():
					timer.Stop()

					return
				case <-timer.C:
				}
			}

			var getResultsRes *http.Response

			getResultsRes, err = cfg.Client.NewRequest().
				URL(getResultsReqURL).
				Param("k", intelXKey).
				Param("id", searchResData.ID).
				Param("limit", "10000").
				Do(ctx)
			if err != nil {
				if getResultsRes != nil {
					_ = getResultsRes.Body.Close()
				}

				result := sources.Result{
					Type:   sources.ResultError,
					Source: s.Name(),
					Error:  fmt.Errorf("request failed: %w", sources.RedactError(err, intelXKey)),
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

				_ = getResultsRes.Body.Close()

				return
			}

			_ = getResultsRes.Body.Close()

			status = getResultsResData.Status

			for _, selector := range getResultsResData.Selectors {
				var URL string

				var valid bool

				if URL, valid = cfg.Validator(selector.Selectvalue); !valid {
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

var _ sources.Source = (*Source)(nil)

const (
	defaultPollInterval    = 10 * time.Second
	defaultPollAttemptsMax = 30
	defaultSearchTimeout   = 20
)

func New() (source sources.Source) {
	source = &Source{
		keys:            make(sources.Keys, 0),
		pollInterval:    defaultPollInterval,
		pollAttemptsMax: defaultPollAttemptsMax,
	}

	return
}
