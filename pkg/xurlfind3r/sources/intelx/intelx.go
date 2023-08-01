package intelx

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type searchRequest struct {
	Term       string        `json:"term"`
	Timeout    time.Duration `json:"timeout"`
	Target     int           `json:"target"`
	MaxResults int           `json:"maxresults"`
	Media      int           `json:"media"`
}
type searchResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type getResultsResponse struct {
	Selectors []struct {
		Selectvalue string `json:"selectorvalue"`
	} `json:"selectors"`
	Status int `json:"status"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var err error

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

		searchReqURL := fmt.Sprintf("https://%s/phonebook/search?k=%s", intelXHost, intelXKey)
		searchReqBody := searchRequest{
			Term:       "*" + domain,
			MaxResults: 100000,
			Media:      0,
			Target:     3, // 1 = Domains | 2 = Emails | 3 = URLs
			Timeout:    20,
		}

		var searchReqBodyBytes []byte

		searchReqBodyBytes, err = json.Marshal(searchReqBody)
		if err != nil {
			return
		}

		var searchRes *fasthttp.Response

		searchRes, err = httpclient.SimplePost(searchReqURL, "application/json", searchReqBodyBytes)
		if err != nil {
			return
		}

		var searchResData searchResponse

		if err = json.Unmarshal(searchRes.Body(), &searchResData); err != nil {
			return
		}

		getResultsReqURL := fmt.Sprintf("https://%s/phonebook/search/result?k=%s&id=%s&limit=10000", intelXHost, intelXKey, searchResData.ID)
		status := 0

		for status == 0 || status == 3 {
			var getResultsRes *fasthttp.Response

			getResultsRes, err = httpclient.Get(getResultsReqURL, "", nil)
			if err != nil {
				return
			}

			var getResultsResData getResultsResponse

			if err = json.Unmarshal(getResultsRes.Body(), &getResultsResData); err != nil {
				return
			}

			status = getResultsResData.Status

			for _, hostname := range getResultsResData.Selectors {
				URL := hostname.Selectvalue
				URL = sources.FixURL(URL)

				parsedURL, err := hqgourl.Parse(URL)
				if err != nil {
					return
				}

				parsedURL.Path = strings.Split(parsedURL.Path, ":")[0]

				URL = parsedURL.String()

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					continue
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
