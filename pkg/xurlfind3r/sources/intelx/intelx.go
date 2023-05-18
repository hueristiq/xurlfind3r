// Package intelx implements functions to search URLs from intelx.
package intelx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
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

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			err  error
			body []byte
			res  *fasthttp.Response
		)

		if config.Keys.IntelXKey == "" || config.Keys.IntelXHost == "" {
			return
		}

		searchURL := fmt.Sprintf("https://%s/phonebook/search?k=%s", config.Keys.IntelXHost, config.Keys.IntelXKey)
		reqBody := requestBody{
			Term:       config.Domain,
			MaxResults: 100000,
			Media:      0,
			Timeout:    20,
		}

		body, err = json.Marshal(reqBody)
		if err != nil {
			return
		}

		res, err = httpclient.SimplePost(searchURL, "application/json", body)
		if err != nil {
			return
		}

		var response searchResponseType

		if err = json.Unmarshal(res.Body(), &response); err != nil {
			return
		}

		resultsURL := fmt.Sprintf("https://%s/phonebook/search/result?k=%s&id=%s&limit=10000", config.Keys.IntelXHost, config.Keys.IntelXKey, response.ID)
		status := 0

		for status == 0 || status == 3 {
			res, err = httpclient.Get(resultsURL, "", nil)
			if err != nil {
				return
			}

			var response searchResultType

			if err = json.Unmarshal(res.Body(), &response); err != nil {
				return
			}

			status = response.Status

			for _, hostname := range response.Selectors {
				URL := hostname.Selectvalue

				if !sources.IsValid(URL) {
					continue
				}

				if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "intelx"
}
