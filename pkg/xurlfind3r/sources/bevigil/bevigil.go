package bevigil

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type getURLsResponse struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var err error

		var key string

		key, err = sources.PickRandom(config.Keys.Bevigil)
		if key == "" || err != nil {
			return
		}

		reqHeaders := map[string]string{}

		if len(config.Keys.Bevigil) > 0 {
			reqHeaders["X-Access-Token"] = key
		}

		reqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", domain)

		var res *fasthttp.Response

		res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", reqHeaders, nil)
		if err != nil {
			return
		}

		var getURLsResponseData getURLsResponse

		if err = json.Unmarshal(res.Body(), &getURLsResponseData); err != nil {
			return
		}

		for _, URL := range getURLsResponseData.URLs {
			if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
				return
			}

			URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "bevigil"
}
