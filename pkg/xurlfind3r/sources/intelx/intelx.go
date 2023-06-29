// Package intelx implements functions to search URLs from intelx.
package intelx

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type SearchResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type searchResultType struct {
	Selectors []selectorType `json:"selectors"`
	Status    int            `json:"status"`
}

type selectorType struct {
	Selectvalue string `json:"selectorvalue"`
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
			err error
		)

		var key string

		key, err = sources.PickRandom(config.Keys.Intelx)
		if key == "" || err != nil {
			return
		}

		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			return
		}

		intelXHost := parts[0]
		intelXKey := parts[1]

		if intelXKey == "" || intelXHost == "" {
			return
		}

		searchURL := fmt.Sprintf("https://%s/phonebook/search?k=%s", intelXHost, intelXKey)
		searchReqBody := requestBody{
			Term:       config.Domain,
			MaxResults: 100000,
			Media:      0,
			Timeout:    20,
		}

		var body []byte

		body, err = json.Marshal(searchReqBody)
		if err != nil {
			return
		}

		var res *fasthttp.Response

		res, err = httpclient.SimplePost(searchURL, "application/json", body)
		if err != nil {
			return
		}

		var resData SearchResponse

		if err = json.Unmarshal(res.Body(), &resData); err != nil {
			return
		}

		resultsURL := fmt.Sprintf("https://%s/phonebook/search/result?k=%s&id=%s&limit=10000", intelXHost, intelXKey, resData.ID)
		status := 0

		for status == 0 || status == 3 {
			res, err = httpclient.Get(resultsURL, "", nil)
			if err != nil {
				return
			}

			var resData searchResultType

			if err = json.Unmarshal(res.Body(), &resData); err != nil {
				return
			}

			status = resData.Status

			for _, hostname := range resData.Selectors {
				URL := hostname.Selectvalue

				if isEmail(URL) {
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

func isEmail(URL string) (isEmail bool) {
	_, err := mail.ParseAddress(URL)
	isEmail = err == nil

	return
}

func (source *Source) Name() string {
	return "intelx"
}
