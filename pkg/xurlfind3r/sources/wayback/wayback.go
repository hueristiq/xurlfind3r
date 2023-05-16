package wayback

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/valyala/fasthttp"
)

type Source struct{}

func (source *Source) Run(config sources.Configuration, domain string) (URLs chan sources.URL) {
	URLs = make(chan sources.URL)

	go func() {
		defer close(URLs)

		var (
			err error
			res *fasthttp.Response
		)

		if config.IncludeSubdomains {
			domain = "*." + domain
		}

		res, err = httpclient.SimpleGet(fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&sources=txt&fl=original&collapse=urlkey", domain))
		if err != nil {
			return
		}

		scanner := bufio.NewScanner(bytes.NewReader(res.Body()))

		for scanner.Scan() {
			URL := scanner.Text()
			if URL == "" {
				continue
			}

			URL, err = url.QueryUnescape(URL)
			if err != nil {
				return
			}

			if URL != "" {
				URL = strings.ToLower(URL)
				URL = strings.TrimPrefix(URL, "25")
				URL = strings.TrimPrefix(URL, "2f")

				URLs <- sources.URL{Source: source.Name(), Value: URL}
			}
		}
	}()

	return
}

func (source *Source) Name() string {
	return "wayback"
}
