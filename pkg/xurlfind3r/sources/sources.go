package sources

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"
)

type Configuration struct {
	Client    *hqgohttp.Client
	Extractor *regexp.Regexp
	Validator func(target string) (URL string, valid bool)
}

type ResultType int

type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

type Source interface {
	Name() (name string)
	NeedsKeys() (needs bool)
	UseKeys(keys ...string)
	Run(ctx context.Context, cfg *Configuration, domain string) <-chan Result
}

type redactedError struct {
	err     error
	message string
}

func (e *redactedError) Error() (message string) {
	return e.message
}

func (e *redactedError) Unwrap() (err error) {
	return e.err
}

type Keys []string

func (k Keys) PickRandom() (key string, err error) {
	length := len(k)

	if length == 0 {
		err = errors.New("no keys configured")

		return
	}

	maximum := big.NewInt(int64(length))

	indexBig, err := rand.Int(rand.Reader, maximum)
	if err != nil {
		err = fmt.Errorf("failed to generate random index for key selection: %w", err)

		return
	}

	index := indexBig.Int64()

	key = k[index]

	if key == "" {
		err = errors.New("selected key is empty")

		return
	}

	return
}

const (
	ResultURL ResultType = iota + 1
	ResultError
	ResultWarning
)

const MaxPages = 100

const (
	Arquivo            = "arquivo"
	Bevigil            = "bevigil"
	Commoncrawl        = "commoncrawl"
	Ghostarchive       = "ghostarchive"
	Github             = "github"
	Gitlab             = "gitlab"
	Hudsonrock         = "hudsonrock"
	IntelligenceX      = "intelx"
	Leakradar          = "leakradar"
	OpenThreatExchange = "otx"
	URLScan            = "urlscan"
	Virustotal         = "virustotal"
	Wayback            = "wayback"
	Zoomeye            = "zoomeye"
)

var List = []string{
	Arquivo,
	Bevigil,
	Commoncrawl,
	Ghostarchive,
	Github,
	Gitlab,
	Hudsonrock,
	IntelligenceX,
	Leakradar,
	OpenThreatExchange,
	URLScan,
	Virustotal,
	Wayback,
	Zoomeye,
}

var _ error = (*redactedError)(nil)

var (
	gluedSchemePattern = regexp.MustCompile(`[,;|][Hh][Tt]{2}[Pp][Ss]?://`)
	gluedStatusPattern = regexp.MustCompile(`[,;|]\d{3}(?:[,;|]|$)`)
	gluedMethodPattern = regexp.MustCompile(`[,;|][A-Z]{2,8}$`)
)

func splitGluedURLs(match string) (URLs []string) {
	glue := gluedSchemePattern.FindAllStringIndex(match, -1)

	if len(glue) == 0 {
		return []string{match}
	}

	URLs = make([]string, 0, len(glue)+1)

	trim := func(URL string) (trimmed string) {
		trimmed = URL

		if statuses := gluedStatusPattern.FindAllStringIndex(URL, -1); len(statuses) > 0 {
			trimmed = URL[:statuses[len(statuses)-1][0]]

			return
		}

		if methods := gluedMethodPattern.FindStringIndex(URL); methods != nil {
			trimmed = URL[:methods[0]]
		}

		return
	}

	start := 0

	for _, indexes := range glue {
		URL := match[start:indexes[0]]
		start = indexes[0] + 1

		if URL = trim(URL); URL != "" {
			URLs = append(URLs, URL)
		}
	}

	if URL := trim(match[start:]); URL != "" {
		URLs = append(URLs, URL)
	}

	return
}

func ExtractURLs(extractor *regexp.Regexp, text string) (URLs []string) {
	URLs = make([]string, 0)

	for _, indexes := range extractor.FindAllStringIndex(text, -1) {
		start := indexes[0]

		if start > 0 && isHostLabelCharacter(text[start-1]) {
			continue
		}

		end := indexes[1]

		if end < len(text) && isHostLabelCharacter(text[end]) {
			if text[end] != '.' || end+1 < len(text) && isHostLabelCharacter(text[end+1]) {
				continue
			}
		}

		URLs = append(URLs, splitGluedURLs(text[start:end])...)
	}

	return
}

func ExtractURLsAll(extractor *regexp.Regexp, rawText string) (URLs []string) {
	seen := make(map[string]struct{})

	collect := func(text string) {
		for _, URL := range ExtractURLs(extractor, text) {
			if _, exists := seen[URL]; exists {
				continue
			}

			seen[URL] = struct{}{}

			URLs = append(URLs, URL)
		}
	}

	collect(rawText)

	if decoded := hqgourlextractor.DecodeJSEscapes(html.UnescapeString(rawText)); decoded != rawText {
		collect(decoded)
	}

	return
}

func ExtractCollapsedURLs(extractor *regexp.Regexp, text string) (URLs []string) {
	for _, group := range hqgourlextractor.JSLiteralGroups(text) {
		if len(group.Pieces) == 1 && !group.HasExpr {
			continue
		}

		collapsed := group.Collapsed()

		if strings.Trim(collapsed, hqgourlextractor.ExpressionPlaceholder+"/") == "" {
			continue
		}

		URLs = append(URLs, ExtractURLs(extractor, collapsed)...)
	}

	return
}

func SendURLs(results chan<- Result, cfg *Configuration, sourceName string, URLs []string) {
	for _, URL := range URLs {
		URL, valid := cfg.Validator(URL)
		if !valid {
			continue
		}

		results <- Result{
			Type:   ResultURL,
			Source: sourceName,
			Value:  URL,
		}
	}
}

func isHostLabelCharacter(character byte) (ok bool) {
	switch {
	case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9', character == '.', character == '-', character == '_':
		ok = true
	}

	return
}

func RedactError(err error, secrets ...string) (redacted error) {
	if err == nil || len(secrets) == 0 {
		return err
	}

	message := err.Error()

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		message = strings.ReplaceAll(message, secret, "[REDACTED]")
		message = strings.ReplaceAll(message, url.QueryEscape(secret), "[REDACTED]")
	}

	if message == err.Error() {
		return err
	}

	return &redactedError{
		err:     err,
		message: message,
	}
}

func CheckResponse(res *http.Response, secrets ...string) (err error) {
	if res == nil {
		err = errors.New("nil response")

		return
	}

	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return
	}

	message := "unexpected status " + res.Status

	if res.Body != nil {
		var snippet []byte

		snippet, _ = io.ReadAll(io.LimitReader(res.Body, 4<<10))

		if body := strings.TrimSpace(string(snippet)); body != "" {
			message += ": " + body
		}
	}

	err = RedactError(errors.New(message), secrets...)

	return
}

func ErrorResult(source string, err error) (result Result) {
	result = Result{
		Type:   ResultError,
		Source: source,
		Error:  err,
	}

	return
}

func WarningResult(source, value string) (result Result) {
	result = Result{
		Type:   ResultWarning,
		Source: source,
		Value:  value,
	}

	return
}

func URLResult(source, value string) (result Result) {
	result = Result{
		Type:   ResultURL,
		Source: source,
		Value:  value,
	}

	return
}
