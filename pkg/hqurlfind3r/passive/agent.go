// Package passive provides capability for doing passive subdomain
// enumeration on targets.
package passive

import (
	"regexp"
	"sync"

	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/commoncrawl"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/github"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/intelx"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/otx"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/urlscan"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/wayback"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/scraping/sources/waybackrobots"
	"github.com/hueristiq/hqurlfind3r/pkg/hqurlfind3r/session"
)

// Agent is a struct for running passive url collection for a given host.
// It wraps scraping package and provides a layer to build upon.
type Agent struct {
	sources map[string]scraping.Source
}

// New creates a new agent for passive url collection
// Create the agent, insert the sources and remove the excluded sources
func New(sourcesToUse, sourcesToExclude []string) (agent *Agent) {
	agent = &Agent{
		sources: make(map[string]scraping.Source),
	}

	agent.addSources(sourcesToUse)
	agent.removeSources(sourcesToExclude)

	return
}

// Run collects all the known urls for a given domain
func (agent *Agent) Run(domain string, filterRegex *regexp.Regexp, includeSubdomains bool, keys session.Keys) (URLs chan scraping.URL) {
	URLs = make(chan scraping.URL)

	go func() {
		defer close(URLs)

		ses, err := session.New(domain, filterRegex, includeSubdomains, 10, keys)
		if err != nil {
			return
		}

		wg := &sync.WaitGroup{}

		for name, source := range agent.sources {
			wg.Add(1)

			go func(name string, source scraping.Source) {
				defer wg.Done()

				for res := range source.Run(domain, ses, includeSubdomains) {
					URLs <- res
				}
			}(name, source)
		}

		wg.Wait()
	}()

	return
}

// addSources adds the given list of sources to the source array
func (agent *Agent) addSources(sourcesToUse []string) {
	for _, source := range sourcesToUse {
		switch source {
		case "commoncrawl":
			agent.sources[source] = &commoncrawl.Source{}
		case "github":
			agent.sources[source] = &github.Source{}
		case "intelx":
			agent.sources[source] = &intelx.Source{}
		case "otx":
			agent.sources[source] = &otx.Source{}
		case "urlscan":
			agent.sources[source] = &urlscan.Source{}
		case "wayback":
			agent.sources[source] = &wayback.Source{}
		case "waybackrobots":
			agent.sources[source] = &waybackrobots.Source{}
		}
	}
}

// removeSources deletes the given sources from the source map
func (agent *Agent) removeSources(sourcesToExclude []string) {
	for _, source := range sourcesToExclude {
		delete(agent.sources, source)
	}
}
