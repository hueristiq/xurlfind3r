package urlscan

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/session"
)

type response struct {
	Results []struct {
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

type Source struct{}

func (source *Source) Run(domain string, ses *session.Session, includeSubs bool) chan scraping.URL {
	URLs := make(chan scraping.URL)

	go func() {
		defer close(URLs)

		res, err := ses.SimpleGet(fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s", domain))
		if err != nil {
			return
		}

		body := res.Body()

		var results response

		if err := json.Unmarshal(body, &results); err != nil {
			return
		}

		for _, i := range results.Results {
			if URL, ok := scraping.NormalizeURL(i.Page.URL, ses.Scope); ok {
				URLs <- scraping.URL{Source: source.Name(), Value: URL}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "urlscan"
}
