package intelx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/filter"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/output"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/requests"
	"github.com/hueristiq/xurlfind3r/pkg/runner/collector/sources"
	"github.com/valyala/fasthttp"
)

type searchResponseType struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type selectorType struct {
	Selectvalue string `json:"selectorvalue"`
}

type searchResultType struct {
	Selectors []selectorType `json:"selectors"`
	Status    int            `json:"status"`
}

type requestBody struct {
	Term       string        `json:"term"`
	Timeout    time.Duration `json:"timeout"`
	MaxResults int           `json:"maxresults"`
	Media      int           `json:"media"`
}

type Source struct{}

func (source *Source) Run(keys sources.Keys, ftr filter.Filter) (URLs chan output.URL) {
	domain := ftr.Domain

	URLs = make(chan output.URL)

	go func() {
		defer close(URLs)

		var (
			err  error
			body []byte
			res  *fasthttp.Response
		)

		if keys.IntelXKey == "" || keys.IntelXHost == "" {
			return
		}

		searchURL := fmt.Sprintf("https://%s/phonebook/search?k=%s", keys.IntelXHost, keys.IntelXKey)
		reqBody := requestBody{
			Term:       domain,
			MaxResults: 100000,
			Media:      0,
			Timeout:    20,
		}

		body, err = json.Marshal(reqBody)
		if err != nil {
			return
		}

		res, err = requests.SimplePost(searchURL, "application/json", body)
		if err != nil {
			return
		}

		var response searchResponseType

		if err = json.Unmarshal(res.Body(), &response); err != nil {
			return
		}

		resultsURL := fmt.Sprintf("https://%s/phonebook/search/result?k=%s&id=%s&limit=10000", keys.IntelXHost, keys.IntelXKey, response.ID)
		status := 0

		for status == 0 || status == 3 {
			res, err = requests.Get(resultsURL, "", nil)
			if err != nil {
				return
			}

			var response searchResultType

			if err = json.Unmarshal(res.Body(), &response); err != nil {
				return
			}

			status = response.Status

			for _, hostname := range response.Selectors {
				if URL, ok := ftr.Examine(hostname.Selectvalue); ok {
					URLs <- output.URL{Source: source.Name(), Value: URL}
				}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "intelx"
}
