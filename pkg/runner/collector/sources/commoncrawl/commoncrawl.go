package commoncrawl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/output"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/requests"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/sources"
)

type Source struct{}

type CDXAPIResult struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type CommonCrawlIndex struct {
	ID      string `json:"id"`
	CDX_API string `json:"cdx-api"`
}

func (source *Source) Run(keys sources.Keys, ftr filter.Filter) chan output.URL {
	domain := ftr.Domain

	URLs := make(chan output.URL)

	go func() {
		defer close(URLs)

		res, err := requests.SimpleGet("https://index.commoncrawl.org/collinfo.json")
		if err != nil {
			return
		}

		var commonCrawlIndexes []CommonCrawlIndex

		if err := json.Unmarshal(res.Body(), &commonCrawlIndexes); err != nil {
			fmt.Println(err)

			return
		}

		wg := new(sync.WaitGroup)

		for index := range commonCrawlIndexes {
			commonCrawlIndex := commonCrawlIndexes[index]

			wg.Add(1)

			API := commonCrawlIndex.CDX_API

			go func(api string) {
				defer wg.Done()

				var headers = map[string]string{"Host": "index.commoncrawl.org"}

				res, err := requests.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", api, domain), "", headers)
				if err != nil {
					return
				}

				sc := bufio.NewScanner(bytes.NewReader(res.Body()))

				for sc.Scan() {
					var result CDXAPIResult

					if err := json.Unmarshal(sc.Bytes(), &result); err != nil {
						return
					}

					if result.Error != "" {
						return
					}

					if URL, ok := ftr.Examine(result.URL); ok {
						URLs <- output.URL{Source: source.Name(), Value: URL}
					}
				}
			}(API)
		}

		wg.Wait()
	}()

	return URLs
}

func (source *Source) Name() string {
	return "commoncrawl"
}
