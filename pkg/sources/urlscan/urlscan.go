package urlscan

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/enenumxela/urlx/pkg/urlx"
	"github.com/signedsecurity/sigurlfind3r/pkg/session"
	"github.com/signedsecurity/sigurlfind3r/pkg/sources"
)

type response struct {
	Results []struct {
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

type Source struct{}

func (source *Source) Run(domain string, ses *session.Session, includeSubs bool) chan sources.URLs {
	URLs := make(chan sources.URLs)

	go func() {
		defer close(URLs)

		res, err := ses.SimpleGet(fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s", domain))
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

		for _, i := range results.Results {
			parsedURL, err := urlx.Parse(i.Page.URL)
			if err != nil {
				continue
			}

			if parsedURL.ETLDPlus1 == domain {
				if includeSubs {
					URLs <- sources.URLs{Source: source.Name(), Value: i.Page.URL}
				} else {
					if parsedURL.SubDomain == "" || parsedURL.SubDomain == "www" {
						URLs <- sources.URLs{Source: source.Name(), Value: i.Page.URL}
					}
				}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "urlscan"
}
