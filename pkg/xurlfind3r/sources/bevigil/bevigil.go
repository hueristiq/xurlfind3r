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

		getURLsReqHeaders := map[string]string{}

		if len(config.Keys.Bevigil) > 0 {
			getURLsReqHeaders["X-Access-Token"] = key
		}

		getURLsReqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", domain)

		var getURLsRes *fasthttp.Response

		getURLsRes, err = httpclient.Get(getURLsReqURL, "", getURLsReqHeaders)
		if err != nil {
			return
		}

		var getURLsResData getURLsResponse

		if err = json.Unmarshal(getURLsRes.Body(), &getURLsResData); err != nil {
			return
		}

		for _, URL := range getURLsResData.URLs {
			if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
				continue
			}

			URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "bevigil"
}
