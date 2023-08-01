package otx

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
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

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		parseURL, err := hqgourl.Parse(domain)
		if err != nil {
			return
		}

		for page := 1; ; page++ {
			getURLsReqURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=100&page=%d", parseURL.ETLDPlusOne, page)

			var err error

			var getURLsRes *fasthttp.Response

			getURLsRes, err = httpclient.SimpleGet(getURLsReqURL)
			if err != nil {
				return
			}

			var getURLsResData getURLsResponse

			if err = json.Unmarshal(getURLsRes.Body(), &getURLsResData); err != nil {
				return
			}

			for _, item := range getURLsResData.URLList {
				URL := item.URL

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					continue
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}

			if !getURLsResData.HasNext {
				break
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "otx"
}
