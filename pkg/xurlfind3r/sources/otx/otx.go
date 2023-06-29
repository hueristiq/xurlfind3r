// Package otx implements functions to search URLs from otx.
package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type Response struct {
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

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		for page := 1; ; page++ {
			var (
				err error
			)

			reqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=%d&page=%d", config.Domain, 200, page)

			var res *fasthttp.Response

			res, err = httpclient.SimpleGet(reqURL)
			if err != nil {
				return
			}

			var resData Response

			if err = json.Unmarshal(res.Body(), &resData); err != nil {
				return
			}

			for index := range resData.URLList {
				URL := resData.URLList[index].URL

				if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}

			if !resData.HasNext {
				break
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "otx"
}
