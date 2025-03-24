package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	hqgohttp "go.source.hueristiq.com/http"
)

type getURLsResponse struct {
	URLList []struct {
		URL      string `json:"url"`
		Domain   string `json:"domain"`
		Hostname string `json:"hostname"`
		Result   struct {
			URLWorker struct {
				IP       string `json:"ip"`
				HTTPCode int    `json:"http_code"`
			} `json:"urlworker"`
		} `json:"result"`
		HTTPCode int    `json:"httpcode"`
		Encoded  string `json:"encoded"`
	} `json:"url_list"`
	PageNum    int  `json:"page_num"`
	Limit      int  `json:"limit"`
	Paged      bool `json:"paged"`
	HasNext    bool `json:"has_next"`
	FullSize   int  `json:"full_size"`
	ActualSize int  `json:"actual_size"`
}

type Source struct{}

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		for page := 1; ; page++ {
			getURLsReqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list", domain)
			getURLsReqCFG := &hqgohttp.RequestConfiguration{
				Params: map[string]string{
					"limit": "100",
					"page":  fmt.Sprintf("%d", page),
				},
			}

			getURLsRes, err := hqgohttp.Get(getURLsReqURL, getURLsReqCFG)
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

			for _, item := range getURLsResData.URLList {
				URL := item.URL

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

			if !getURLsResData.HasNext {
				break
			}
		}
	}()

	return results
}

func (source *Source) Name() string {
	return sources.OPENTHREATEXCHANGE
}
