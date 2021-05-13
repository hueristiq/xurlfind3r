package otx

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/signedsecurity/sigurlfind3r/pkg/session"
	"github.com/signedsecurity/sigurlfind3r/pkg/sources"
)

type Source struct{}

type response struct {
	HasNext    bool `json:"has_next"`
	ActualSize int  `json:"actual_size"`
	URLList    []struct {
		Domain   string `json:"domain"`
		URL      string `json:"url"`
		Hostname string `json:"hostname"`
		HTTPCode int    `json:"httpcode"`
		PageNum  int    `json:"page_num"`
		FullSize int    `json:"full_size"`
		Paged    bool   `json:"paged"`
	} `json:"url_list"`
}

func (source *Source) Run(domain string, ses *session.Session, includeSubs bool) chan sources.URLs {
	URLs := make(chan sources.URLs)

	go func() {
		defer close(URLs)

		for page := 0; ; page++ {
			res, err := ses.SimpleGet(fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?limit=%d&page=%d", domain, 200, page))
			if err != nil {
				ses.DiscardHTTPResponse(res)
				return
			}

			defer res.Body.Close()

			var results response

			body, err := ioutil.ReadAll(res.Body)

			if err := json.Unmarshal(body, &results); err != nil {
				return
			}

			for _, i := range results.URLList {
				if URL, ok := sources.NormalizeURL(i.URL, ses.Scope); ok {
					URLs <- sources.URLs{Source: source.Name(), Value: URL}
				}
			}

			if !results.HasNext {
				break
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "otx"
}
