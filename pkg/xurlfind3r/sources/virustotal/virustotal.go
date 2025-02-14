package virustotal

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	hqgohttp "go.source.hueristiq.com/http"
	"go.source.hueristiq.com/http/method"
)

type getDomainReportResponse struct {
	DetectedURLs []struct {
		URL string `json:"url"`
	} `json:"detected_urls"`
	Subdomains     []string        `json:"subdomains"`
	UndetectedURLs [][]interface{} `json:"undetected_urls"`
}

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := cfg.Keys.VirusTotal.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		getDomainReportReqURL := fmt.Sprintf("https://www.virustotal.com/vtapi/v2/domain/report?apikey=%s&domain=%s", key, domain)

		getDomainReportRes, err := hqgohttp.Request().Method(method.GET.String()).URL(getDomainReportReqURL).Send()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getDomainReportResData getDomainReportResponse

		if err = json.NewDecoder(getDomainReportRes.Body).Decode(&getDomainReportResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getDomainReportRes.Body.Close()

			return
		}

		getDomainReportRes.Body.Close()

		for _, detectedURL := range getDomainReportResData.DetectedURLs {
			URL := detectedURL.URL

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

		for _, subdomain := range getDomainReportResData.Subdomains {
			URL := "http://" + subdomain

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

		for _, undetectedURL := range getDomainReportResData.UndetectedURLs {
			if len(undetectedURL) > 0 {
				if URL, ok := undetectedURL[0].(string); ok {
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
		}
	}()

	return results
}

func (source *Source) Name() string {
	return sources.VIRUSTOTAL
}
