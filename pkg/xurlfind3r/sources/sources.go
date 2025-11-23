package sources

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
)

type Source interface {
	Name() (name string)
	UseKeys(keys ...string)
	Run(cfg *Configuration, domain string) <-chan Result
}

type Configuration struct {
	Extractor         *regexp.Regexp
	IncludeSubdomains bool
	Validate          func(target string) (URL string, valid bool)
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
	OPENTHREATEXCHANGE,
	URLSCAN,
	VIRUSTOTAL,
	WAYBACK,
}
