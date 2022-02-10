package sigurlfind3r

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"github.com/enenumxela/urlx/pkg/urlx"
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
	Keys              session.Keys
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
		Options: opts,
	}

	if opts.FilterRegex != "" {
		runner.FilterRegex = regexp.MustCompile(opts.FilterRegex)
	}

	runner.Passive = passive.New(opts.SourcesToUse, opts.SourcesToExclude)

	return
}

// Run runs the url collection flow on the specified target
func (runner *Runner) Run(ctx context.Context, domain string) (URLs chan scraping.URL, err error) {
	URLs = make(chan scraping.URL)

	results := runner.Passive.Run(domain, runner.FilterRegex, runner.Options.IncludeSubdomains, runner.Options.Keys)

	deDupMap := make(map[string]url.Values)
	uniqueMap := make(map[string]scraping.URL)

	// Process the results in a separate goroutine
	go func() {
		defer close(URLs)

		for result := range results {
			// unique urls - If the url already exists in the unique map
			if _, exists := uniqueMap[result.Value]; exists {
				continue
			}

			parsedURL, err := urlx.Parse(result.Value)
			if err != nil {
				continue
			}

			// urls with query
			if len(parsedURL.Query()) > 0 {
				unique := false

				key := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Domain, parsedURL.Path)

				if _, exists := deDupMap[key]; exists {
					for parameter := range parsedURL.Query() {
						if _, exists := deDupMap[key][parameter]; !exists {
							deDupMap[key][parameter] = []string{"sigurlfind3r"}
							unique = true
						}
					}
				} else {
					deDupMap[key] = parsedURL.Query()
					unique = true
				}

				if !unique {
					continue
				}
			}

			uniqueMap[parsedURL.String()] = scraping.URL{
				Source: result.Source,
				Value:  parsedURL.String(),
			}

			URLs <- uniqueMap[parsedURL.String()]
		}
	}()

	return
}
