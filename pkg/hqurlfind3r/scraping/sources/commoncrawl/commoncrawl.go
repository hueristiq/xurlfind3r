package commoncrawl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/session"
)

type Source struct{}

type CommonPaginationResult struct {
	Blocks   uint `json:"blocks"`
	PageSize uint `json:"pageSize"`
	Pages    uint `json:"pages"`
}

type CommonResult struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type CommonAPIResult []struct {
	ID  string `json:"id"`
	API string `json:"cdx-api"`
}

func (source *Source) Run(domain string, ses *session.Session, includeSubs bool) chan scraping.URL {
	URLs := make(chan scraping.URL)

	go func() {
		defer close(URLs)

		res, err := ses.SimpleGet("https://index.commoncrawl.org/collinfo.json")
		if err != nil {
			return
		}

		var apiRresults CommonAPIResult

		if err := json.Unmarshal(res.Body(), &apiRresults); err != nil {
			fmt.Println(err)
			return
		}

		wg := new(sync.WaitGroup)

		for _, u := range apiRresults {
			wg.Add(1)

			go func(api string) {
				defer wg.Done()

				var headers = map[string]string{"Host": "index.commoncrawl.org"}

				res, err := ses.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", api, domain), "", headers)
				if err != nil {
					return
				}

				sc := bufio.NewScanner(bytes.NewReader(res.Body()))

				for sc.Scan() {
					var result CommonResult

					if err := json.Unmarshal(sc.Bytes(), &result); err != nil {
						return
					}

					if result.Error != "" {
						return
					}

					if URL, ok := scraping.NormalizeURL(result.URL, ses.Scope); ok {
						URLs <- scraping.URL{Source: source.Name(), Value: URL}
					}
				}
			}(u.API)
		}

		wg.Wait()
	}()

	return URLs
}

func (source *Source) Name() string {
	return "commoncrawl"
}
