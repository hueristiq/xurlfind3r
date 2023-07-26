package urlscan

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type searchResponse struct {
	Results []struct {
		Page struct {
			Domain   string `json:"domain"`
			MimeType string `json:"mimeType"`
			URL      string `json:"url"`
			Status   string `json:"status"`
		} `json:"page"`
		Sort []interface{} `json:"sort"`
	} `json:"results"`
	Status  int  `json:"status"`
	Total   int  `json:"total"`
	Took    int  `json:"took"`
	HasMore bool `json:"has_more"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var err error

		var key string

		key, err = sources.PickRandom(config.Keys.URLScan)
		if err != nil {
			return
		}

		searchReqHeaders := map[string]string{
			"Content-Type": "application/json",
		}

		if key != "" {
			searchReqHeaders["API-Key"] = key
		}

		var searchAfter []interface{}

		for {
			after := ""

			if searchAfter != nil {
				searchAfterJSON, _ := json.Marshal(searchAfter)
				after = "&search_after=" + string(searchAfterJSON)
			}

			searchReqURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=100", domain) + after

			var searchRes *fasthttp.Response

			searchRes, err = httpclient.Get(searchReqURL, "", searchReqHeaders)
			if err != nil {
				return
			}

			var searchResData searchResponse

			if err = json.Unmarshal(searchRes.Body(), &searchResData); err != nil {
				return
			}

			if searchResData.Status == 429 {
				break
			}

			for _, result := range searchResData.Results {
				if (result.Page.Domain != domain && result.Page.Domain != "www."+domain) &&
					(config.IncludeSubdomains && !strings.HasSuffix(result.Page.Domain, domain)) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: result.Page.URL}
			}

			if !searchResData.HasMore {
				break
			}

			lastResult := searchResData.Results[len(searchResData.Results)-1]
			searchAfter = lastResult.Sort
		}
	}()

	return
}

func (source *Source) Name() string {
	return "urlscan"
}
