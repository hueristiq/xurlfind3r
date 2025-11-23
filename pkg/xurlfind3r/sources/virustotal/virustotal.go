package virustotal

import (
	"encoding/json"
	"fmt"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgolimiter "github.com/hueristiq/hq-go-limiter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getDomainReportResponse struct {
	DetectedURLs []struct {
		URL string `json:"url"`
	} `json:"detected_urls"`
	Subdomains     []string        `json:"subdomains"`
	UndetectedURLs [][]interface{} `json:"undetected_urls"`
}

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() (name string) {
	name = sources.VIRUSTOTAL

	return
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
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

		getDomainReportReqURL := "https://www.virustotal.com/vtapi/v2/domain/report"
		getDomainReportReqCFG := &hqgohttp.RequestConfiguration{
			Params: map[string]string{
				"apikey": key,
				"domain": domain,
			},
		}

		limiter.Wait()

		getDomainReportRes, err := hqgohttp.Get(getDomainReportReqURL, getDomainReportReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("request failed: %w", err),
			}

			results <- result

			return
		}

		var getDomainReportResData getDomainReportResponse

		if err = json.NewDecoder(getDomainReportRes.Body).Decode(&getDomainReportResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: s.Name(),
				Error:  fmt.Errorf("failed to parse JSON response: %w", err),
			}

			results <- result

			getDomainReportRes.Body.Close()

			return
		}

		getDomainReportRes.Body.Close()

		for _, detectedURL := range getDomainReportResData.DetectedURLs {
			var URL string

			var valid bool

			if URL, valid = cfg.Validate(detectedURL.URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: s.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, subdomain := range getDomainReportResData.Subdomains {
			var URL string

			var valid bool

			if URL, valid = cfg.Validate(subdomain); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: s.Name(),
				Value:  URL,
			}

			results <- result
		}

		for _, undetectedURL := range getDomainReportResData.UndetectedURLs {
			if len(undetectedURL) > 0 {
				if URL, ok := undetectedURL[0].(string); ok {
					var valid bool

					if URL, valid = cfg.Validate(URL); !valid {
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
		}
	}()

	return results
}

var limiter = hqgolimiter.New(&hqgolimiter.Configuration{
	RequestsPerMinute:     4,
	MinimumDelayInSeconds: 30,
})

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{
		keys: make(sources.Keys, 0),
	}

	return
}
