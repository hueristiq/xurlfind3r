// Package sources provides the core interfaces, types, and constants required for integrating
// multiple data sources.
//
// This package defines the Source interface which every data source implementation must satisfy.
// It standardizes the functionality for URL discovery, ensuring consistent behavior across
// various integrations. In addition, the package provides configuration types for managing API keys,
// regular expression extractors, and other settings needed for interacting with external data sources.
// The Result and ResultType types are used to encapsulate the outcomes of data collection operations,
// making it easy to report successful URL discoveries or errors.
//
// Supported data sources are defined by a set of constants (e.g., BEVIGIL, COMMONCRAWL, GITHUB, etc.) and a
// List slice that can be used to iterate over or validate available integrations.
package sources

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
)

// Source is the interface that every data source implementation must satisfy.
// It standardizes the required functionality, ensuring uniform behavior across
// different integrations. Implementers of Source must define two methods:
//
//   - Run: Initiates data collection for a given domain using the provided configuration,
//     returning results asynchronously through a channel.
//   - Name: Returns the unique identifier (name) of the data source for logging and reporting.
type Source interface {
	// Run initiates the data collection or scanning process for a specified domain.
	// The method accepts a domain name and a pointer to a Configuration instance,
	// and returns a read-only channel through which results (of type Result) are streamed.
	//
	// Parameters:
	//   - domain (string): A string representing the target domain for data collection.
	//   - cfg (*Configuration): A pointer to a Configuration struct containing API keys, regular expressions,
	//          and any other settings needed for interacting with the data source.
	//
	// Returns:
	//   - (<-chan Result): A read-only channel that asynchronously emits Result values,
	//     allowing the caller to process subdomain data or errors as they become available.
	Run(domain string, cfg *Configuration) <-chan Result

	// Name returns the unique name of the data source.
	//
	// This identifier is used for distinguishing among multiple data sources,
	// especially when logging activity or compiling results from several integrations.
	//
	// Returns:
	//   - name (string): A string that uniquely identifies the data source.
	Name() (name string)
}

// Configuration holds settings and parameters passed to each data source.
//
// Fields:
//   - Keys (Keys): API credentials for different data sources.
//   - IncludeSubdomains (bool): Whether subdomains should be considered in scope.
//   - Extractor (*regexp.Regexp): A compiled regular expression used to extract URLs.
//   - Validate (func(string) (string, bool)): A custom function that determines
//     if a target is in scope and optionally transforms it.
type Configuration struct {
	Keys              Keys
	IncludeSubdomains bool
	Extractor         *regexp.Regexp
	Validate          func(target string) (URL string, valid bool)
}

// Keys stores API keys for different data sources. Each field represents a collection of API keys
// for a specific source, and is defined using the SourceKeys type (a slice of strings). These keys are
// used for authentication when interacting with external APIs or services.
type Keys struct {
	Bevigil    SourceKeys `yaml:"bevigil"`
	Github     SourceKeys `yaml:"github"`
	IntelX     SourceKeys `yaml:"intelx"`
	URLScan    SourceKeys `yaml:"urlscan"`
	VirusTotal SourceKeys `yaml:"virustotal"`
}

// SourceKeys is a slice of strings where each element represents an API key for a specific source.
// This structure supports maintaining multiple keys for a single source, which is useful for key
// rotation or providing fallback options if one key becomes invalid.
type SourceKeys []string

// PickRandom selects and returns a random API key from the SourceKeys slice.
//
// It uses a cryptographically secure RNG (rand.Reader) to prevent predictable
// selection. This is particularly useful for evenly distributing usage
// across multiple keys or avoiding rate limits.
//
// Returns:
//   - key (string): A randomly chosen key from the slice.
//   - err (error): An error if the slice is empty or if secure RNG fails.
func (k SourceKeys) PickRandom() (key string, err error) {
	length := len(k)

	if length == 0 {
		err = ErrNoKeys

		return
	}

	maximum := big.NewInt(int64(length))

	var indexBig *big.Int

	indexBig, err = rand.Int(rand.Reader, maximum)
	if err != nil {
		err = fmt.Errorf("failed to generate random index: %w", err)

		return
	}

	index := indexBig.Int64()

	key = k[index]

	return
}

// Result represents the outcome of URL discovery.
// It encapsulates details about the result, including its type, the originating source,
// the actual data (if available), and any error encountered during the operation.
//
// Fields:
//   - Type (ResultType): Specifies the kind of result (e.g., URL or error).
//   - Source (string): Identifies the source that produced this result (e.g., "bevigil", "github").
//   - Value (string): Contains the actual URL retrieved from the source.
//     This field is empty if the result is an error.
//   - Error (error): Holds the error encountered during the operation, if any. If no error
//     occurred, this field is nil.
type Result struct {
	Type   ResultType
	Source string
	Value  string
	Error  error
}

// ResultType defines the category of a Result using an integer enumeration.
// It allows for distinguishing between different types of outcomes produced by sources.
//
// Enumeration Values:
//   - ResultURL: Indicates a successful result containing a URL retrieved from the source.
//   - ResultError: Represents a result indicating that an error occurred during the operation.
type ResultType int

// Constants representing the types of results that can be produced by a data source.
//
// List of Constants:
//   - ResultURL: Represents a successful result containing URL.
//   - ResultError: Indicates an error encountered during the operation, with details
//     provided in the `Error` field of the `Result`.
const (
	ResultURL ResultType = iota
	ResultError
)

// Supported data source constants.
//
// The following constants define the names of supported data sources.
// Each constant is used as a unique identifier for its corresponding data source.
const (
	BEVIGIL            = "bevigil"
	COMMONCRAWL        = "commoncrawl"
	GITHUB             = "github"
	INTELLIGENCEX      = "intelx"
	OPENTHREATEXCHANGE = "otx"
	URLSCAN            = "urlscan"
	VIRUSTOTAL         = "virustotal"
	WAYBACK            = "wayback"
)

// ErrNoKeys is a sentinel error returned when a SourceKeys slice contains no API keys.
// This error is used to signal that an operation requiring an API key cannot proceed
// because no keys are available.
var ErrNoKeys = errors.New("no keys available for the source")

// List is a collection of all supported source names.
//
// This slice provides a convenient way to iterate over, validate, or dynamically configure
// the data sources available in the application.
var List = []string{
	BEVIGIL,
	COMMONCRAWL,
	GITHUB,
	INTELLIGENCEX,
	OPENTHREATEXCHANGE,
	URLSCAN,
	VIRUSTOTAL,
	WAYBACK,
}
