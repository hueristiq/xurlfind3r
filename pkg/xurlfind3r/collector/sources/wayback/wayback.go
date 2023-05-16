package wayback

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/filter"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/collector/sources"
	"github.com/valyala/fasthttp"
)

type Source struct{}

func (source *Source) Run(_ sources.Keys, ftr filter.Filter) chan sources.URL {
	domain := ftr.Domain

	URLs := make(chan sources.URL)

	go func() {
		defer close(URLs)

		var (
			err error
			res *fasthttp.Response
		)

		if ftr.IncludeSubdomains {
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

				var ok bool

				if URL, ok = ftr.Examine(URL); ok {
					URLs <- sources.URL{Source: source.Name(), Value: URL}
				}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "wayback"
}
