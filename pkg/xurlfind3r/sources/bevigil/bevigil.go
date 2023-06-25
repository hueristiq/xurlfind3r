package bevigil

import (
	"encoding/json"
	"fmt"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type response struct {
	Domain string   `json:"domain"`
	URLs   []string `json:"urls"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			key     string
			err     error
			res     *fasthttp.Response
			headers = map[string]string{}
		)

		key, err = sources.PickRandom(config.Keys.Bevigil)
		if key == "" || err != nil {
			return
		}

		if len(config.Keys.Bevigil) > 0 {
			headers["X-Access-Token"] = key
		}

		reqURL := fmt.Sprintf("https://osint.bevigil.com/api/%s/urls/", config.Domain)

		res, err = httpclient.Request(fasthttp.MethodGet, reqURL, "", headers, nil)
		if err != nil {
			return
		}

		body := res.Body()

		var results response

		if err = json.Unmarshal(body, &results); err != nil {
			return
		}

		for _, i := range results.URLs {
			URLsChannel <- sources.URL{Source: source.Name(), Value: i}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "bevigil"
}
