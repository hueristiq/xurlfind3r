package commoncrawl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
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

		res, err := ses.SimpleGet("http://index.commoncrawl.org/collinfo.json")
		if err != nil {
			ses.DiscardHTTPResponse(res)
			return
		}

		var apiRresults CommonAPIResult

		body, err := ioutil.ReadAll(res.Body)

		if err := json.Unmarshal(body, &apiRresults); err != nil {
			return
		}

		res.Body.Close()

		for _, u := range apiRresults {
			var headers = map[string]string{"Host": "index.commoncrawl.org"}

			res, err := ses.Get(fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", u.API, domain), headers)
			if err != nil {
				ses.DiscardHTTPResponse(res)
				continue
			}

			defer res.Body.Close()

			sc := bufio.NewScanner(res.Body)

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
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "commoncrawl"
}
