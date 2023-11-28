package otx

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
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

func (source *Source) Run(config *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		parseURL, err := hqgourl.Parse(domain)
		if err != nil {
			result := sources.Result{
				Type:   sources.Error,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			return
		}

		for page := 1; ; page++ {
			getURLsReqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=100&page=%d", parseURL.ETLDPlusOne, page)

			var err error

			var getURLsRes *http.Response

			getURLsRes, err = httpclient.SimpleGet(getURLsReqURL)
			if err != nil {
				result := sources.Result{
					Type:   sources.Error,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				httpclient.DiscardResponse(getURLsRes)

				return
			}

			var getURLsResData getURLsResponse

			if err = json.NewDecoder(getURLsRes.Body).Decode(&getURLsResData); err != nil {
				result := sources.Result{
					Type:   sources.Error,
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

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					continue
				}

				result := sources.Result{
					Type:   sources.URL,
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
	return "otx"
}
