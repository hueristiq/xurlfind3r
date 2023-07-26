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

type indexesResponse []struct {
	ID  string `json:"id"`
	API string `json:"cdx-API"`
}

type response struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		var indexesRes *fasthttp.Response
		var err error

		indexesRes, err = httpclient.SimpleGet("https://index.commoncrawl.org/collinfo.json")
		if err != nil {
			return
		}

		var indexesResponseData indexesResponse

		if err = json.Unmarshal(indexesRes.Body(), &indexesResponseData); err != nil {
			return
		}

		wg := new(sync.WaitGroup)

		for _, indexData := range indexesResponseData {
			wg.Add(1)

			go func(API string) {
				defer wg.Done()

				contentReqHeaders := map[string]string{
					"Host": "index.commoncrawl.org",
				}

				var contentRes *fasthttp.Response
				var err error

				contentRes, err = httpclient.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", API, domain), "", contentReqHeaders)
				if err != nil {
					return
				}

				scanner := bufio.NewScanner(bytes.NewReader(contentRes.Body()))

				for scanner.Scan() {
					var data response

					if err = json.Unmarshal(scanner.Bytes(), &data); err != nil {
						return
					}

					if data.Error != "" {
						return
					}

					URL := data.URL

					if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
						return
					}

					URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
				}

				if scanner.Err() != nil {
					return
				}
			}(indexData.API)
		}

		wg.Wait()
	}()

	return
}

func (source *Source) Name() string {
	return "commoncrawl"
}
