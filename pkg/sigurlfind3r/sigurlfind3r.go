package sigurlfind3r

import (
	"context"
	"regexp"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/passive"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

// Runner is an instance of url collection client used
// to orchestrate the whole process.
type Runner struct {
	FilterRegex *regexp.Regexp
	Passive     *passive.Agent
	Options     *Options
}

// Options is an instance of options used in creation of a Runner.
type Options struct {
	FilterRegex       string
	SourcesToExclude  []string
	SourcesToUse      []string
	IncludeSubdomains bool
	Keys              *session.Keys
}

// New creates a new Runner struct instance by parsing
// the configuration options, configuring sources, etc.
func New(options ...*Options) (runner *Runner) {
	// Set default options
	opts := &Options{
		FilterRegex:       "",
		IncludeSubdomains: false,
		SourcesToExclude:  make([]string, 0),
		SourcesToUse:      scraping.SourcesList,
	}

	// Override default options if custom provided
	if len(options) > 0 {
		opts = options[0]
	}

	runner = &Runner{
		FilterRegex: regexp.MustCompile(opts.FilterRegex),
		Passive:     passive.New(opts.SourcesToUse, opts.SourcesToExclude),
		Options:     opts,
	}

	return
}

// Run runs the url collection flow on the specified target
func (runner *Runner) Run(ctx context.Context, domain string) (URLs chan scraping.URL, err error) {
	URLs = make(chan scraping.URL)

	// Create a unique map for filtering duplicate URLs out
	uniqueMap := make(map[string]scraping.URL)
	// Create a map to track sources for each URL
	sourceMap := make(map[string]map[string]struct{})

	results := runner.Passive.Run(domain, runner.FilterRegex, runner.Options.IncludeSubdomains, runner.Options.Keys)

	// Process the results in a separate goroutine
	go func() {
		defer close(URLs)

		for result := range results {
			URL := result.Value

			if _, exists := uniqueMap[URL]; !exists {
				sourceMap[URL] = make(map[string]struct{})
			}

			sourceMap[URL][result.Source] = struct{}{}

			if _, exists := uniqueMap[URL]; exists {
				continue
			}

			hostEntry := scraping.URL{Source: result.Source, Value: URL}

			uniqueMap[URL] = hostEntry

			URLs <- hostEntry
		}
	}()

	return
}
