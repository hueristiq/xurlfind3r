package hudsonrock

import (
	"encoding/json"
	"fmt"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type getURLsResponse struct {
	Data struct {
		EmployeesUrls []struct {
			URL string `json:"url"`
		} `json:"employees_urls"`
		ClientsUrls []struct {
			URL string `json:"url"`
		} `json:"clients_urls"`
	} `json:"data"`
}

type Source struct{}

func (s *Source) Name() (name string) {
	name = sources.HUDSONROCK

	return
}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		getURLsReqURL := "https://cavalier.hudsonrock.com/api/json/v2/osint-tools/urls-by-domain"
		getURLsReqCFG := &hqgohttp.RequestConfiguration{
			Params: map[string]string{
				"domain": domain,
			},
		}

		getURLsRes, err := hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  fmt.Errorf("request failed: %w", err),
			}

			results <- result

			return
		}

		var getURLsResData getURLsResponse

		if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  fmt.Errorf("failed to parse JSON response: %w", err),
			}

			results <- result

			getURLsRes.Body.Close()

			return
		}

		getURLsRes.Body.Close()

		for _, record := range append(getURLsResData.Data.EmployeesUrls, getURLsResData.Data.ClientsUrls...) {
			URL := record.URL

			var valid bool

			if URL, valid = cfg.Validator(URL); !valid {
				continue
			}

			result := sources.Result{
				Type:   sources.ResultURL,
				Source: source.Name(),
				Value:  URL,
			}

			results <- result
		}
	}()

	return results
}

func (s *Source) UseKeys(keys ...string) {
}

var _ sources.Source = (*Source)(nil)

func New() (source sources.Source) {
	source = &Source{}

	return
}
