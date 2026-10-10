package sources

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
)

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
	BEVIGIL            = "bevigil"
	COMMONCRAWL        = "commoncrawl"
	GITHUB             = "github"
	HUDSONROCK         = "hudsonrock"
	INTELLIGENCEX      = "intelx"
	LEAKRADAR          = "leakradar"
	OPENTHREATEXCHANGE = "otx"
	URLSCAN            = "urlscan"
	VIRUSTOTAL         = "virustotal"
	WAYBACK            = "wayback"
)

var List = []string{
	BEVIGIL,
	COMMONCRAWL,
	GITHUB,
	HUDSONROCK,
	INTELLIGENCEX,
	LEAKRADAR,
	OPENTHREATEXCHANGE,
	URLSCAN,
	VIRUSTOTAL,
	WAYBACK,
}
