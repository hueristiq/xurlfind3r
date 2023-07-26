// Package otx implements functions to search URLs from otx.
package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type response struct {
	URLList []struct {
		Domain   string `json:"domain"`
		URL      string `json:"url"`
		Hostname string `json:"hostname"`
		HTTPCode int    `json:"httpcode"`
		PageNum  int    `json:"page_num"`
		FullSize int    `json:"full_size"`
		Paged    bool   `json:"paged"`
	} `json:"url_list"`
	PageNum    int  `json:"page_num"`
	Paged      bool `json:"paged"`
	HasNext    bool `json:"has_next"`
	FullSize   int  `json:"full_size"`
	ActualSize int  `json:"actual_size"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		for page := 1; ; page++ {
			reqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=%d&page=%d", domain, 200, page)

			var err error

			var res *fasthttp.Response

			res, err = httpclient.SimpleGet(reqURL)
			if err != nil {
				return
			}

			var responseData response

			if err = json.Unmarshal(res.Body(), &responseData); err != nil {
				return
			}

			for _, URL := range responseData.URLList {
				if !sources.IsInScope(URL.URL, domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL.URL}
			}

			if !responseData.HasNext {
				break
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "otx"
}
