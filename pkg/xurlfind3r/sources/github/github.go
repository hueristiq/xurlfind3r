package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/hueristiq/hqgourl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
	"github.com/tomnomnom/linkheader"
	"github.com/valyala/fasthttp"
)

type searchResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		TextMatches []struct {
			Fragment string `json:"fragment"`
		} `json:"text_matches"`
	} `json:"items"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration, domain string) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		if len(config.Keys.GitHub) == 0 {
			return
		}

		tokens := NewTokenManager(config.Keys.GitHub)

		searchReqURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%q&sort=created&order=asc", domain)

		source.Enumerate(searchReqURL, domain, tokens, URLsChannel, config)
	}()

	return URLsChannel
}

func (source *Source) Enumerate(searchReqURL, domain string, tokens *Tokens, URLsChannel chan sources.URL, config *sources.Configuration) {
	token := tokens.Get()

	if token.RetryAfter > 0 {
		if len(tokens.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = tokens.Get()
		}
	}

	searchReqHeaders := map[string]string{
		"Accept":        "application/vnd.github.v3.text-match+json",
		"Authorization": "token " + token.Hash,
	}

	var err error

	var searchRes *fasthttp.Response

	searchRes, err = httpclient.Get(searchReqURL, "", searchReqHeaders)

	isForbidden := searchRes != nil && searchRes.StatusCode() == fasthttp.StatusForbidden

	if err != nil && !isForbidden {
		return
	}

	ratelimitRemaining := cast.ToInt64(searchRes.Header.Peek("X-Ratelimit-Remaining"))
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(searchRes.Header.Peek("Retry-After"))

		tokens.setCurrentTokenExceeded(retryAfterSeconds)

		source.Enumerate(searchReqURL, domain, tokens, URLsChannel, config)
	}

	var searchResData searchResponse

	if err = json.Unmarshal(searchRes.Body(), &searchResData); err != nil {
		return
	}

	var mdExtractor *regexp.Regexp

	mdExtractor, err = hqgourl.Extractor.ModerateMatchHost(`(\w[a-zA-Z0-9][a-zA-Z0-9-\\.]*\.)?` + regexp.QuoteMeta(domain))
	if err != nil {
		return
	}

	for _, item := range searchResData.Items {
		getRawContentReqURL := getRawContentURL(item.HTMLURL)

		var getRawContentRes *fasthttp.Response

		getRawContentRes, err = httpclient.SimpleGet(getRawContentReqURL)
		if err != nil {
			continue
		}

		if getRawContentRes.StatusCode() != fasthttp.StatusOK {
			continue
		}

		URLs := mdExtractor.FindAllString(string(getRawContentRes.Body()), -1)

		for _, URL := range URLs {
			URL = sources.FixURL(URL)

			parsedURL, err := hqgourl.Parse(URL)
			if err != nil {
				return
			}

			URL = parsedURL.String()

			if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
				continue
			}

			URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
		}

		for _, textMatch := range item.TextMatches {
			URLs := mdExtractor.FindAllString(textMatch.Fragment, -1)

			for _, URL := range URLs {
				URL = sources.FixURL(URL)

				parsedURL, err := hqgourl.Parse(URL)
				if err != nil {
					return
				}

				URL = parsedURL.String()

				if !sources.IsInScope(URL, domain, config.IncludeSubdomains) {
					continue
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}
		}
	}

	linksHeader := linkheader.Parse(string(searchRes.Header.Peek("Link")))

	for _, link := range linksHeader {
		if link.Rel == "next" {
			nextURL, err := url.QueryUnescape(link.URL)
			if err != nil {
				return
			}

			source.Enumerate(nextURL, domain, tokens, URLsChannel, config)
		}
	}
}

func (source *Source) Name() string {
	return "github"
}
