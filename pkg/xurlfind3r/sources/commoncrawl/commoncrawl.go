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

type Source struct{}

type CDXAPIResult struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type Index struct {
	ID      string `json:"id"`
	CDX_API string `json:"cdx-API"` //nolint:revive,stylecheck // Is as is
}

func (source *Source) Run(_ sources.Configuration, domain string) (URLs chan sources.URL) {
	URLs = make(chan sources.URL)

	go func() {
		defer close(URLs)

		var (
			err error
			res *fasthttp.Response
		)

		res, err = httpclient.SimpleGet("https://index.commoncrawl.org/collinfo.json")
		if err != nil {
			return
		}

		var commonCrawlIndexes []Index

		if err = json.Unmarshal(res.Body(), &commonCrawlIndexes); err != nil {
			return
		}

		wg := new(sync.WaitGroup)

		for index := range commonCrawlIndexes {
			wg.Add(1)

			commonCrawlIndex := commonCrawlIndexes[index]

			go func(API string) {
				defer wg.Done()

				var (
					err     error
					headers = map[string]string{"Host": "index.commoncrawl.org"}
					res     *fasthttp.Response
				)

				res, err = httpclient.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", API, domain), "", headers)
				if err != nil {
					return
				}

				scanner := bufio.NewScanner(bytes.NewReader(res.Body()))

				for scanner.Scan() {
					var result CDXAPIResult

					if err = json.Unmarshal(scanner.Bytes(), &result); err != nil {
						return
					}

					if result.Error != "" {
						return
					}

					URLs <- sources.URL{Source: source.Name(), Value: result.URL}
				}
			}(commonCrawlIndex.CDX_API)
		}

		wg.Wait()
	}()

	return
}

func (source *Source) Name() string {
	return "commoncrawl"
}
