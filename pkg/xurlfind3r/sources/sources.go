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
	ResultURL ResultType = iota + 1
	ResultError
)

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
