package sources

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"
)

// URLExtractor binds a target domain's host pattern to the shared extractor
// implementation. Sources pass the value carried in
// [Configuration.Extractor] to [ExtractURLs], [ExtractURLsAll], and
// [ExtractCollapsedURLs].
type URLExtractor struct {
	options []hqgourlextractor.OptionFunc
}

// NewURLExtractor creates an extractor restricted to domain and its
// subdomains, without email addresses or user info.
func NewURLExtractor(domain string) (extractor *URLExtractor) {
	label := `[a-z0-9_](?:[a-z0-9_-]*[a-z0-9_])?`
	pattern := `(?i)(?:(?:` + label + `[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.PortOptionalPattern + `)`

	return &URLExtractor{
		options: []hqgourlextractor.OptionFunc{
			hqgourlextractor.WithHostPattern(pattern),
			hqgourlextractor.WithoutEmail(),
			hqgourlextractor.WithoutUserInfo(),
		},
	}
}

// NewURLExtractorFromRegexp creates an extractor around a precompiled
// pattern. It keeps the boundary and glue cleanup behavior of [ExtractURLs].
func NewURLExtractorFromRegexp(re *regexp.Regexp) (extractor *URLExtractor) {
	return &URLExtractor{
		options: []hqgourlextractor.OptionFunc{
			hqgourlextractor.WithRegexp(re),
		},
	}
}

// Regexp compiles the extractor's configured pattern.
func (e *URLExtractor) Regexp() (regex *regexp.Regexp, err error) {
	extractor, err := hqgourlextractor.New(e.options...)
	if err != nil {
		return nil, err //nolint:wrapcheck // pass-through
	}

	return extractor.Regexp(), nil
}

type Configuration struct {
	Client    *hqgohttp.Client
	Extractor *URLExtractor
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

func ExtractURLs(extractor *URLExtractor, text string) (URLs []string) {
	if extractor == nil {
		return nil
	}

	URLs, _ = hqgourlextractor.ExtractURLs(text, extractor.options...)

	return
}

func ExtractURLsAll(extractor *URLExtractor, rawText string) (URLs []string) {
	if extractor == nil {
		return nil
	}

	URLs, _ = hqgourlextractor.ExtractURLsAll(rawText, extractor.options...)

	return
}

func ExtractCollapsedURLs(extractor *URLExtractor, text string) (URLs []string) {
	if extractor == nil {
		return nil
	}

	URLs, _ = hqgourlextractor.ExtractCollapsedURLs(text, extractor.options...)

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
