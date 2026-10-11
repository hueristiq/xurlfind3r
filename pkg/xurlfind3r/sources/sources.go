package sources

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
)

// RedactError returns err with every occurrence of each secret replaced
// by "[REDACTED]" in its message, preserving the error chain for
// errors.Is and errors.As.
//
// Sources that authenticate through query parameters (e.g. API keys in
// request URLs) use it to keep keys out of error results, which may be
// logged or printed.
//
// Parameters:
//   - err (error): the error whose message may embed secrets.
//   - secrets (...string): the secrets to redact.
//
// Returns:
//   - redacted (error): the redacted error, or err unchanged when it
//     contains none of the secrets.
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
	}

	if message == err.Error() {
		return err
	}

	return &redactedError{
		err:     err,
		message: message,
	}
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

var _ error = (*redactedError)(nil)

// ExtractURLs extracts candidate URLs from text using extractor.
//
// Matches whose end is immediately followed by a host-label character
// ([a-zA-Z0-9._-]) are dropped: they are prefixes of longer hosts
// (e.g. example.com inside example.com.evil.com) and would otherwise
// surface URLs that never existed.
//
// Parameters:
//   - extractor (*regexp.Regexp): the compiled URL extractor.
//   - text (string): the text to scan for URLs.
//
// Returns:
//   - URLs ([]string): the extracted URLs.
func ExtractURLs(extractor *regexp.Regexp, text string) (URLs []string) {
	URLs = make([]string, 0)

	for _, indexes := range extractor.FindAllStringIndex(text, -1) {
		end := indexes[1]

		if end < len(text) && isHostLabelCharacter(text[end]) {
			continue
		}

		URLs = append(URLs, text[indexes[0]:end])
	}

	return
}

// isHostLabelCharacter reports whether character can continue a host
// label (letters, digits, dot, hyphen or underscore).
func isHostLabelCharacter(character byte) (ok bool) {
	switch {
	case character >= 'a' && character <= 'z',
		character >= 'A' && character <= 'Z',
		character >= '0' && character <= '9',
		character == '.', character == '-', character == '_':
		ok = true
	}

	return
}

type Source interface {
	Name() (name string)
	Run(ctx context.Context, cfg *Configuration, domain string) <-chan Result
	NeedsKeys() (needs bool)
	UseKeys(keys ...string)
}

type Configuration struct {
	Client    *hqgohttp.Client
	Extractor *regexp.Regexp
	Validator func(target string) (URL string, valid bool)
}

type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

type ResultType int

type Keys []string

func (k Keys) PickRandom() (key string, err error) {
	length := len(k)

	if length == 0 {
		err = errors.New("no keys configured")

		return
	}

	maximum := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, maximum)
	if err != nil {
		err = fmt.Errorf("failed to generate random index for key selection: %w", err)

		return
	}

	index := indexBig.Int64()

	key = k[index]

	return
}

const (
	ResultURL ResultType = iota
	ResultError
)

const (
	ARQUIVO            = "arquivo"
	BEVIGIL            = "bevigil"
	COMMONCRAWL        = "commoncrawl"
	GHOSTARCHIVE       = "ghostarchive"
	GITHUB             = "github"
	GITLAB             = "gitlab"
	HUDSONROCK         = "hudsonrock"
	INTELLIGENCEX      = "intelx"
	LEAKRADAR          = "leakradar"
	OPENTHREATEXCHANGE = "otx"
	URLSCAN            = "urlscan"
	VIRUSTOTAL         = "virustotal"
	WAYBACK            = "wayback"
	ZOOMEYE            = "zoomeye"
)

var List = []string{
	ARQUIVO,
	BEVIGIL,
	COMMONCRAWL,
	GHOSTARCHIVE,
	GITHUB,
	GITLAB,
	HUDSONROCK,
	INTELLIGENCEX,
	LEAKRADAR,
	OPENTHREATEXCHANGE,
	URLSCAN,
	VIRUSTOTAL,
	WAYBACK,
	ZOOMEYE,
}
