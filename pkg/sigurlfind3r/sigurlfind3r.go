package sigurlfind3r

import (
	"context"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/passive"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

type Keys struct {
	GitHub []string `json:"github"`
}

type Options struct {
	SourcesToExclude []string
	SourcesToUse     []string
	Keys             *session.Keys
}

type Runner struct {
	Options *Options
	Passive *passive.Agent
}

func New(options ...*Options) (runner *Runner) {
	// Set default config
	opts := &Options{
		SourcesToExclude: make([]string, 0),
		SourcesToUse:     scraping.SourcesList,
	}

	// Override config if provided
	if len(options) > 0 {
		opts = options[0]
	}

	runner = &Runner{
		Options: opts,
		Passive: passive.New(opts.SourcesToUse, opts.SourcesToExclude),
	}

	return
}

func (runner *Runner) Run(ctx context.Context, domain string, subs bool) (URLs chan scraping.URL, err error) {
	URLs = make(chan scraping.URL)

	// Create a unique map for filtering duplicate URLs out
	uniqueMap := make(map[string]scraping.URL)
	// Create a map to track sources for each URL
	sourceMap := make(map[string]map[string]struct{})

	// keys := runner.options.YAMLConfig.GetKeys()
	results := runner.Passive.Enumerate(domain, subs, runner.Options.Keys)

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
