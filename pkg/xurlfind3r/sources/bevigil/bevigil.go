package bevigil

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type Response struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			err error
			key string
		)

		key, err = sources.PickRandom(config.Keys.Bevigil)
		if key == "" || err != nil {
			return
		}

		reqHeaders := map[string]string{}

		if len(config.Keys.Bevigil) > 0 {
			reqHeaders["X-Access-Token"] = key
		}

		reqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", config.Domain)

		var res *fasthttp.Response

		res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", reqHeaders, nil)
		if err != nil {
			return
		}

		var data Response

		if err = json.Unmarshal(res.Body(), &data); err != nil {
			return
		}

		for index := range data.URLs {
			URL := data.URLs[index]

			URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "bevigil"
}
