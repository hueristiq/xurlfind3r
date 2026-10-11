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

// Source is a passive source of URLs for a domain.
type Source interface {
	// Name returns the source's registered name.
	Name() (name string)

	// Run queries the source for URLs belonging to domain and streams
	// the results.
	Run(ctx context.Context, cfg *Configuration, domain string) <-chan Result

	// NeedsKeys reports whether the source requires API keys.
	NeedsKeys() (needs bool)

	// UseKeys supplies the source with its API keys.
	UseKeys(keys ...string)
}

// Configuration carries the shared plumbing every source consumes.
//
// Fields:
//   - Client (*hqgohttp.Client): the HTTP client for source requests.
//   - Extractor (*regexp.Regexp): the compiled URL extractor scoped to
//     the target domain.
//   - Validator (func): normalizes a candidate URL and reports whether
//     it is in scope.
type Configuration struct {
	Client    *hqgohttp.Client
	Extractor *regexp.Regexp
	Validator func(target string) (URL string, valid bool)
}

// Result is a single outcome of a source run: a discovered URL or an
// error.
//
// Fields:
//   - Type (ResultType): the outcome kind.
//   - Source (string): the source that produced the result.
//   - Value (string): the URL, for URL results.
//   - Error (error): the failure, for error results.
type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

// ResultType distinguishes URL results from error results.
type ResultType int

const (
	// ResultURL marks a discovered URL.
	ResultURL ResultType = iota + 1

	// ResultError marks a source failure.
	ResultError
)

// Keys is a set of API keys a source can pick from.
type Keys []string

// PickRandom returns a uniformly random key from the set.
//
// Returns:
//   - key (string): the selected key.
//   - err (error): non-nil when the set is empty or randomness fails.
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
	// Arquivo is the registered name of the arquivo source.
	Arquivo = "arquivo"

	// Bevigil is the registered name of the bevigil source.
	Bevigil = "bevigil"

	// Commoncrawl is the registered name of the commoncrawl source.
	Commoncrawl = "commoncrawl"

	// Ghostarchive is the registered name of the ghostarchive source.
	Ghostarchive = "ghostarchive"

	// Github is the registered name of the github source.
	Github = "github"

	// Gitlab is the registered name of the gitlab source.
	Gitlab = "gitlab"

	// Hudsonrock is the registered name of the hudsonrock source.
	Hudsonrock = "hudsonrock"

	// IntelligenceX is the registered name of the intelx source.
	IntelligenceX = "intelx"

	// Leakradar is the registered name of the leakradar source.
	Leakradar = "leakradar"

	// OpenThreatExchange is the registered name of the otx source.
	OpenThreatExchange = "otx"

	// URLScan is the registered name of the urlscan source.
	URLScan = "urlscan"

	// Virustotal is the registered name of the virustotal source.
	Virustotal = "virustotal"

	// Wayback is the registered name of the wayback source.
	Wayback = "wayback"

	// Zoomeye is the registered name of the zoomeye source.
	Zoomeye = "zoomeye"
)

// List holds the registered source names in canonical order.
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
