package runner

import (
	"strings"

	"github.com/signedsecurity/sigurlfind3r/pkg/agent"
	"github.com/signedsecurity/sigurlfind3r/pkg/sources"
)

type Runner struct {
	options *Options
	agent   *agent.Agent
}

func New(options *Options) *Runner {
	var uses, exclusions []string

	if options.SourcesUse != "" {
		uses = append(uses, strings.Split(options.SourcesUse, ",")...)
	} else {
		uses = append(uses, sources.All...)
	}

	if options.SourcesExclude != "" {
		exclusions = append(exclusions, strings.Split(options.SourcesExclude, ",")...)
	}

	return &Runner{
		options: options,
		agent:   agent.New(uses, exclusions),
	}
}

func (runner *Runner) Run() (chan sources.URLs, error) {
	URLs := make(chan sources.URLs)

	uniqueMap := make(map[string]sources.URLs)
	sourceMap := make(map[string]map[string]struct{})

	keys := runner.options.YAMLConfig.GetKeys()
	results := runner.agent.Run(runner.options.Domain, keys, runner.options.IncludeSubs)

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

			hostEntry := sources.URLs{Source: result.Source, Value: URL}

			uniqueMap[URL] = hostEntry

			URLs <- hostEntry
		}
	}()

	return URLs, nil
}
