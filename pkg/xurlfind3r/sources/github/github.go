// Package github implements functions to search URLsChannel from github.
package github

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/httpclient"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/tomnomnom/linkheader"
	"github.com/valyala/fasthttp"
)

type Source struct{}

type textMatch struct {
	Fragment string `json:"fragment"`
}

type item struct {
	Name        string      `json:"name"`
	HTMLURL     string      `json:"html_url"`
	TextMatches []textMatch `json:"text_matches"`
}

type response struct {
	TotalCount int    `json:"total_count"`
	Items      []item `json:"items"`
}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		if len(config.Keys.GitHub) == 0 {
			return
		}

		tokens := NewTokenManager(config.Keys.GitHub)

		searchURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%s&sort=created&order=asc", config.Domain)

		source.Enumerate(searchURL, config.URLsRegex, tokens, URLsChannel, config)
	}()

	return URLsChannel
}

func (source *Source) Enumerate(searchURL string, domainRegexp *regexp.Regexp, tokens *Tokens, URLsChannel chan sources.URL, config *sources.Configuration) {
	token := tokens.Get()

	if token.RetryAfter > 0 {
		if len(tokens.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = tokens.Get()
		}
	}

	var (
		err     error
		headers = map[string]string{
			"Accept":        "application/vnd.github.v3.text-match+json",
			"Authorization": "token " + token.Hash,
		}
		res *fasthttp.Response
	)

	res, err = httpclient.Request(fasthttp.MethodGet, searchURL, "", headers, nil)

	isForbidden := res != nil && res.StatusCode() == fasthttp.StatusForbidden

	if err != nil && !isForbidden {
		return
	}

	ratelimitRemaining, _ := strconv.ParseInt(string(res.Header.Peek("X-Ratelimit-Remaining")), 10, 64)
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds, _ := strconv.ParseInt(string(res.Header.Peek("Retry-After")), 10, 64)
		tokens.setCurrentTokenExceeded(retryAfterSeconds)

		source.Enumerate(searchURL, domainRegexp, tokens, URLsChannel, config)
	}

	var results response

	if err = json.Unmarshal(res.Body(), &results); err != nil {
		return
	}

	err = proccesItems(results.Items, domainRegexp, source.Name(), URLsChannel, config)
	if err != nil {
		return
	}

	linksHeader := linkheader.Parse(string(res.Header.Peek("Link")))

	for _, link := range linksHeader {
		if link.Rel == "next" {
			nextURL, err := url.QueryUnescape(link.URL)
			if err != nil {
				return
			}

			source.Enumerate(nextURL, domainRegexp, tokens, URLsChannel, config)
		}
	}
}

func proccesItems(items []item, domainRegexp *regexp.Regexp, name string, URLsChannel chan sources.URL, config *sources.Configuration) (err error) {
	for _, item := range items {
		var (
			res *fasthttp.Response
			URL string
		)

		res, err = httpclient.SimpleGet(rawContentURL(item.HTMLURL))
		if err != nil {
			continue
		}

		if res.StatusCode() == fasthttp.StatusOK {
			scanner := bufio.NewScanner(bytes.NewReader(res.Body()))
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				for _, URL = range domainRegexp.FindAllString(normalizeContent(line), -1) {
					// if !sources.IsValid(URL) {
					// 	continue
					// }

					// if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					// 	return
					// }

					URLsChannel <- sources.URL{Source: name, Value: URL}
				}
			}
		}

		for _, textMatch := range item.TextMatches {
			for _, URL = range domainRegexp.FindAllString(normalizeContent(textMatch.Fragment), -1) {
				// if !sources.IsValid(URL) {
				// 	continue
				// }

				URLsChannel <- sources.URL{Source: name, Value: URL}
			}
		}
	}

	return
}

func normalizeContent(content string) string {
	content, _ = url.QueryUnescape(content)
	content = strings.ReplaceAll(content, "\\t", "")
	content = strings.ReplaceAll(content, "\\n", "")

	return content
}

func rawContentURL(URL string) string {
	URL = strings.ReplaceAll(URL, "https://github.com/", "https://raw.githubusercontent.com/")
	URL = strings.ReplaceAll(URL, "/blob/", "/")

	return URL
}

func (source *Source) Name() string {
	return "github"
}
