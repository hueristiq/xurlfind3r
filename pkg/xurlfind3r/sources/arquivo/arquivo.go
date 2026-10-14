package arquivo

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type cdxRecord struct {
	URL string `json:"url"`
}

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.Arquivo

	return
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		getURLsReqURL := "https://arquivo.pt/wayback/cdx"

		getURLsRes, err := cfg.Client.NewRequest().
			URL(getURLsReqURL).
			Param("url", "*."+domain+"/*").
			Param("output", "json").
			Param("fl", "url").
			Param("collapse", "urlkey").
			Param("limit", "100000").
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

		scanner := bufio.NewScanner(getURLsRes.Body)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				continue
			}

			var record cdxRecord

			if err = json.Unmarshal([]byte(line), &record); err != nil {
				continue
			}

			var URL string

			var valid bool

			if URL, valid = cfg.Validator(record.URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: s.Name(),
				Value:  URL,
			}

			results <- result
		}

		if err = scanner.Err(); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to read response body: %w", err),
			}

			results <- result

			_ = getURLsRes.Body.Close()

			return
		}

		_ = getURLsRes.Body.Close()
	}()

	return results
}

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
