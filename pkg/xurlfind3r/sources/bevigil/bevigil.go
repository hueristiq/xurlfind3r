package bevigil

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	hqgohttp "go.source.hueristiq.com/http"
)

type getURLsResponse struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		key, err := cfg.Keys.Bevigil.PickRandom()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getURLsRes *http.Response

		getURLsReqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", domain)
		getURLsReqCFG := &hqgohttp.RequestConfiguration{
			Headers: map[string]string{
				"X-Access-Token": key,
			},
		}

		getURLsRes, err = hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		var getURLsResData getURLsResponse

		if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getURLsRes.Body.Close()

			return
		}

		getURLsRes.Body.Close()

		for _, URL := range getURLsResData.URLs {
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
	}()

	return results
}

func (source *Source) Name() string {
	return sources.BEVIGIL
}
