package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/filter"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/output"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/requests"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/sources"
	"github.com/valyala/fasthttp"
)

type Source struct{}

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

func (source *Source) Run(_ sources.Keys, ftr filter.Filter) (URLs chan output.URL) {
	domain := ftr.Domain

	URLs = make(chan output.URL)

	go func() {
		defer close(URLs)

		var (
			err error
			res *fasthttp.Response
		)

		for page := 1; ; page++ {
			res, err = requests.SimpleGet(fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=%d&page=%d", domain, 200, page))
			if err != nil {
				return
			}

			var results response

			if err = json.Unmarshal(res.Body(), &results); err != nil {
				return
			}

			for _, i := range results.URLList {
				if URL, ok := ftr.Examine(i.URL); ok {
					URLs <- output.URL{Source: source.Name(), Value: URL}
				}
			}

			if !results.HasNext {
				break
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "otx"
}
