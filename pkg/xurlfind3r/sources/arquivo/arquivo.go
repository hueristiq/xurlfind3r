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

func (s *Source) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (s *Source) UseKeys(keys ...string) {
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		getURLsReqURL := "https://arquivo.pt/wayback/cdx"

		getURLsRes, err := cfg.Client.Request().
			URL(getURLsReqURL).
			Param("url", "*."+domain+"/*").
			Param("output", "json").
			Param("fl", "url").
			Param("collapse", "urlkey").
			Param("limit", "100000").
			Context(ctx).Do()
		if err != nil {
			result := sources.ErrorResult(s.Name(), fmt.Errorf("request failed: %w", err))

			results <- result

			return
		}

		if err = sources.CheckResponse(getURLsRes.Std()); err != nil {
			_ = getURLsRes.Body.Close()

			result := sources.ErrorResult(s.Name(), err)

			results <- result

			return
		}

		done := make(chan struct{})

		go func() {
			defer close(done)

			scanner := bufio.NewScanner(getURLsRes.Body)

			scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

			for scanner.Scan() {
				line := scanner.Text()

				if line == "" {
					continue
				}

				var record cdxRecord

				if err = json.Unmarshal([]byte(line), &record); err != nil {
					continue
				}

				URL, valid := cfg.Validator(record.URL)
				if !valid {
					continue
				}

				result := sources.URLResult(s.Name(), URL)

				results <- result
			}

			if scanErr := scanner.Err(); scanErr != nil && ctx.Err() == nil {
				result := sources.ErrorResult(s.Name(), fmt.Errorf("failed to read response body: %w", scanErr))

				results <- result
			}
		}()

		select {
		case <-done:
		case <-ctx.Done():
			_ = getURLsRes.Body.Close()

			<-done
		}

		_ = getURLsRes.Body.Close()
	}()

	return results
}

const maxScanTokenSize = 1 << 20

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
