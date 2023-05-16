package urlscan

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/filter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/sources"
	"github.com/valyala/fasthttp"
)

type response struct {
	Results []struct {
		Page struct {
			URL string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

type Source struct{}

func (source *Source) Run(_ sources.Keys, ftr filter.Filter) chan sources.URL {
	domain := ftr.Domain

	URLs := make(chan sources.URL)

	go func() {
		defer close(URLs)

		var (
			err error
			res *fasthttp.Response
		)

		res, err = httpclient.SimpleGet(fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s", domain))
		if err != nil {
			return
		}

		body := res.Body()

		var results response

		if err = json.Unmarshal(body, &results); err != nil {
			return
		}

		for _, i := range results.Results {
			if URL, ok := ftr.Examine(i.Page.URL); ok {
				URLs <- sources.URL{Source: source.Name(), Value: URL}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "urlscan"
}
