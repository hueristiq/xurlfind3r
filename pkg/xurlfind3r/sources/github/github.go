package github

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/spf13/cast"
	hqgohttp "go.source.hueristiq.com/http"
	"go.source.hueristiq.com/http/header"
	hqgoheaderparser "go.source.hueristiq.com/http/header/parser"
	"go.source.hueristiq.com/http/method"
	"go.source.hueristiq.com/http/status"
	hqgourlextractor "go.source.hueristiq.com/url/extractor"
	hqgourlparser "go.source.hueristiq.com/url/parser"
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

func (source *Source) Run(cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		if len(cfg.Keys.Github) == 0 {
			return
		}

		tokens := NewTokenManager(cfg.Keys.Github)

		searchReqURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%q&sort=created&order=asc", domain)

		source.Enumerate(searchReqURL, domain, tokens, results, cfg)
	}()

	return results
}

func (source *Source) Enumerate(searchReqURL, domain string, tokens *Tokens, results chan sources.Result, cfg *sources.Configuration) {
	token := tokens.Get()

	if token.RetryAfter > 0 {
		if len(tokens.pool) == 1 {
			time.Sleep(time.Duration(token.RetryAfter) * time.Second)
		} else {
			token = tokens.Get()
		}
	}

	var err error

	var searchRes *http.Response

	searchRes, err = hqgohttp.Request().Method(method.GET.String()).URL(searchReqURL).AddHeader("Accept", "application/vnd.github.v3.text-match+json").AddHeader("Authorization", "token "+token.Hash).Send()

	isForbidden := searchRes != nil && searchRes.StatusCode == status.Forbidden.Int()

	if err != nil && !isForbidden {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: source.Name(),
			Error:  err,
		}

		results <- result

		return
	}

	ratelimitRemaining := cast.ToInt64(searchRes.Header.Get(header.XRatelimitRemaining.String()))
	if isForbidden && ratelimitRemaining == 0 {
		retryAfterSeconds := cast.ToInt64(searchRes.Header.Get(header.RetryAfter.String()))

		tokens.setCurrentTokenExceeded(retryAfterSeconds)

		source.Enumerate(searchReqURL, domain, tokens, results, cfg)
	}

	var searchResData searchResponse

	if err = json.NewDecoder(searchRes.Body).Decode(&searchResData); err != nil {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: source.Name(),
			Error:  err,
		}

		results <- result

		searchRes.Body.Close()

		return
	}

	searchRes.Body.Close()

	mdExtractor := hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.ExtractorPortOptionalPattern + `)`),
	).CompileRegex()

	for _, item := range searchResData.Items {
		getRawContentReqURL := getRawContentURL(item.HTMLURL)

		var getRawContentRes *http.Response

		getRawContentRes, err = hqgohttp.Request().Method(method.GET.String()).URL(getRawContentReqURL).Send()
		if err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			continue
		}

		if getRawContentRes.StatusCode != status.OK.Int() {
			getRawContentRes.Body.Close()

			continue
		}

		scanner := bufio.NewScanner(getRawContentRes.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			URLs := mdExtractor.FindAllString(line, -1)

			for _, URL := range URLs {
				URL = sources.FixURL(URL)

				var parsedURL *hqgourlparser.URL

				parsedURL, err = up.Parse(URL)
				if err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					continue
				}

				URL = parsedURL.String()

				if !cfg.IsInScope(URL) {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}
		}

		if err = scanner.Err(); err != nil {
			result := sources.Result{
				Type:   sources.ResultError,
				Source: source.Name(),
				Error:  err,
			}

			results <- result

			getRawContentRes.Body.Close()

			continue
		}

		getRawContentRes.Body.Close()

		for _, textMatch := range item.TextMatches {
			URLs := mdExtractor.FindAllString(textMatch.Fragment, -1)

			for _, URL := range URLs {
				URL = sources.FixURL(URL)

				parsedURL, err := up.Parse(URL)
				if err != nil {
					result := sources.Result{
						Type:   sources.ResultError,
						Source: source.Name(),
						Error:  err,
					}

					results <- result

					continue
				}

				URL = parsedURL.String()

				if !cfg.IsInScope(URL) {
					continue
				}

				result := sources.Result{
					Type:   sources.ResultURL,
					Source: source.Name(),
					Value:  URL,
				}

				results <- result
			}
		}
	}

	linksHeader := hqgoheaderparser.ParseLinkHeader(searchRes.Header.Get(header.Link.String()))

	for _, link := range linksHeader {
		if link.Rel == "next" {
			nextURL, err := url.QueryUnescape(link.URL)
			if err != nil {
				result := sources.Result{
					Type:   sources.ResultError,
					Source: source.Name(),
					Error:  err,
				}

				results <- result

				return
			}

			source.Enumerate(nextURL, domain, tokens, results, cfg)
		}
	}
}

func (source *Source) Name() string {
	return sources.GITHUB
}

var up = hqgourlparser.NewURLParser(hqgourlparser.URLParserWithDefaultScheme("http"))

func getRawContentURL(htmlURL string) string {
	domain := strings.ReplaceAll(htmlURL, "https://github.com/", "https://raw.githubusercontent.com/")

	return strings.ReplaceAll(domain, "/blob/", "/")
}
