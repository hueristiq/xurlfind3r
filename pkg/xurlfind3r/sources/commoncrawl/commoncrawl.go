// Package commoncrawl implements functions to search URLs from commoncrawl.
package commoncrawl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type API struct {
	ID  string `json:"id"`
	API string `json:"cdx-API"`
}

type Response struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var (
			err error
		)

		var res *fasthttp.Response

		res, err = httpclient.SimpleGet("https://index.commoncrawl.org/collinfo.json")
		if err != nil {
			return
		}

		var APIs []API

		if err = json.Unmarshal(res.Body(), &APIs); err != nil {
			return
		}

		wg := new(sync.WaitGroup)

		for index := range APIs {
			wg.Add(1)

			API := APIs[index]

			go func(API string) {
				defer wg.Done()

				var (
					err error
					// headers = map[string]string{"Host": "index.commoncrawl.org"}
					// res *fasthttp.Response
				)

				reqHeaders := map[string]string{"Host": "index.commoncrawl.org"}

				var res *fasthttp.Response

				res, err = httpclient.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", API, config.Domain), "", reqHeaders)
				if err != nil {
					return
				}

				scanner := bufio.NewScanner(bytes.NewReader(res.Body()))

				for scanner.Scan() {
					var data Response

					if err = json.Unmarshal(scanner.Bytes(), &data); err != nil {
						return
					}

					if data.Error != "" {
						return
					}

					URL := data.URL

					// if !sources.IsValid(URL) {
					// 	return
					// }

					// if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					// 	return
					// }

					URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
				}

				if scanner.Err() != nil {
					return
				}
			}(API.API)
		}

		wg.Wait()
	}()

	return
}

func (source *Source) Name() string {
	return "commoncrawl"
}
