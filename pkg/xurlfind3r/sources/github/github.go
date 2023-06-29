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

type Response struct {
	TotalCount int    `json:"total_count"`
	Items      []Item `json:"items"`
}

type Item struct {
	Name        string      `json:"name"`
	HTMLURL     string      `json:"html_url"`
	TextMatches []TextMatch `json:"text_matches"`
}

type TextMatch struct {
	Fragment string `json:"fragment"`
}

type Source struct{}

func (source *Source) Run(config *sources.Configuration) (URLsChannel chan sources.URL) {
	URLsChannel = make(chan sources.URL)

	go func() {
		defer close(URLsChannel)

		if len(config.Keys.GitHub) == 0 {
			return
		}

		tokens := NewTokenManager(config.Keys.GitHub)

		searchURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%q&sort=created&order=asc", config.Domain)

		source.Enumerate(searchURL, config.URLsRegex, tokens, URLsChannel, config)
	}()

	return URLsChannel
}

func (source *Source) Enumerate(searchURL string, URLsRegex *regexp.Regexp, tokens *Tokens, URLsChannel chan sources.URL, config *sources.Configuration) {
	token := tokens.Get()

	if token.RetryAfter > 0 {
		if len(tokens.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = tokens.Get()
		}
	}

	reqHeaders := map[string]string{
		"Accept":        "application/vnd.github.v3.text-match+json",
		"Authorization": "token " + token.Hash,
	}

	searchRes, err := httpclient.Request(fasthttp.MethodGet, searchURL, "", reqHeaders, nil)

	isForbidden := searchRes != nil && searchRes.StatusCode() == fasthttp.StatusForbidden

	if err != nil && !isForbidden {
		return
	}

	ratelimitRemaining, _ := strconv.ParseInt(string(searchRes.Header.Peek("X-Ratelimit-Remaining")), 10, 64)

	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds, _ := strconv.ParseInt(string(searchRes.Header.Peek("Retry-After")), 10, 64)
		tokens.setCurrentTokenExceeded(retryAfterSeconds)

		source.Enumerate(searchURL, URLsRegex, tokens, URLsChannel, config)
	}

	var searchResData Response

	if err = json.Unmarshal(searchRes.Body(), &searchResData); err != nil {
		return
	}

	// Process Items
	for index := range searchResData.Items {
		item := searchResData.Items[index]

		reqURL := getRawContentURL(item.HTMLURL)

		var contentRes *fasthttp.Response

		contentRes, err = httpclient.SimpleGet(reqURL)
		if err != nil {
			continue
		}

		if contentRes.StatusCode() != fasthttp.StatusOK {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(contentRes.Body()))

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			URLs := URLsRegex.FindAllString(normalizeContent(line), -1)

			for index := range URLs {
				URL := URLs[index]
				URL = fixURL(URL)

				if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					return
				}

				URLsChannel <- sources.URL{Source: source.Name(), Value: URL}
			}
		}

		if scanner.Err() != nil {
			return
		}

		for index := range item.TextMatches {
			textMatch := item.TextMatches[index]

			URLs := URLsRegex.FindAllString(normalizeContent(textMatch.Fragment), -1)

			for index := range URLs {
				URL := URLs[index]
				URL = fixURL(URL)

				if !sources.IsInScope(URL, config.Domain, config.IncludeSubdomains) {
					return
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

			source.Enumerate(nextURL, URLsRegex, tokens, URLsChannel, config)
		}
	}
}

func getRawContentURL(URL string) (rawContentURL string) {
	rawContentURL = URL
	rawContentURL = strings.ReplaceAll(rawContentURL, "https://github.com/", "https://raw.githubusercontent.com/")
	rawContentURL = strings.ReplaceAll(rawContentURL, "/blob/", "/")

	return
}

func normalizeContent(content string) (normalizedContent string) {
	normalizedContent = content
	normalizedContent, _ = url.QueryUnescape(normalizedContent)
	normalizedContent = strings.ReplaceAll(normalizedContent, "\\t", "")
	normalizedContent = strings.ReplaceAll(normalizedContent, "\\n", "")

	return
}

func fixURL(URL string) (fixedURL string) {
	fixedURL = URL

	// ',",`,
	quotes := []rune{'\'', '"', '`'}

	for i := range quotes {
		quote := quotes[i]

		indexOfQuote := findUnbalancedQuote(URL, quote)
		if indexOfQuote <= len(fixedURL) && indexOfQuote >= 0 {
			fixedURL = fixedURL[:indexOfQuote]
		}
	}

	// (),[],{}
	parentheses := []struct {
		Opening, Closing rune
	}{
		{'[', ']'},
		{'(', ')'},
		{'{', '}'},
	}

	for i := range parentheses {
		parenthesis := parentheses[i]

		indexOfParenthesis := findUnbalancedBracket(URL, parenthesis.Opening, parenthesis.Closing)
		if indexOfParenthesis <= len(fixedURL) && indexOfParenthesis >= 0 {
			fixedURL = fixedURL[:indexOfParenthesis]
		}
	}

	// ;
	indexOfComma := strings.Index(fixedURL, ";")
	if indexOfComma <= len(fixedURL) && indexOfComma >= 0 {
		fixedURL = fixedURL[:indexOfComma]
	}

	return
}

func findUnbalancedQuote(s string, quoteChar rune) int {
	insideQuotes := false

	for _, ch := range s {
		if ch == quoteChar {
			if insideQuotes {
				insideQuotes = false
			} else {
				insideQuotes = true
			}
		}
	}

	// If still inside quotes at the end of the string,
	// find the index of the opening quote
	if insideQuotes {
		for i, ch := range s {
			if ch == quoteChar {
				return i
			}
		}
	}

	return -1 // return -1 if all quotes are balanced
}

func findUnbalancedBracket(s string, openChar, closeChar rune) int {
	openCount := 0

	var firstOpenIndex int

	for i, ch := range s {
		if ch == openChar {
			if openCount == 0 {
				firstOpenIndex = i
			}

			openCount++
		} else if ch == closeChar {
			openCount--

			if openCount < 0 {
				return i // Found an unbalanced closing bracket
			}
		}
	}

	// If there are unmatched opening brackets
	if openCount > 0 {
		return firstOpenIndex
	}

	return -1 // All brackets are balanced
}

func (source *Source) Name() string {
	return "github"
}
