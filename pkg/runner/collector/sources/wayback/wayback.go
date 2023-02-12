package wayback

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/output"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/requests"
	"github.com/hueristiq/hqurlfind3r/v2/pkg/runner/collector/sources"
)

type Source struct{}

func (source *Source) Run(keys sources.Keys, ftr filter.Filter) chan output.URL {
	domain := ftr.Domain

	URLs := make(chan output.URL)

	go func() {
		defer close(URLs)

		if ftr.IncludeSubdomains {
			domain = "*." + domain
		}

		res, err := requests.SimpleGet(fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&output=txt&fl=original&collapse=urlkey", domain))
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

				if URL, ok := ftr.Examine(URL); ok {
					URLs <- output.URL{Source: source.Name(), Value: URL}
				}
			}
		}
	}()

	return URLs
}

func (source *Source) Name() string {
	return "wayback"
}
