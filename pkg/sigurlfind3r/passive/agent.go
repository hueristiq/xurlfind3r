package passive

import (
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/commoncrawl"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/github"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/otx"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/urlscan"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/wayback"
)

// Agent is a struct for running passive url collection for a given host.
// It wraps scraping package and provides a layer to build upon.
type Agent struct {
	sources map[string]scraping.Source
}

// New creates a new agent for passive url collection
func New(sourcesToUse, sourcesToExclude []string) (agent *Agent) {
	// Create the agent, insert the sources and remove the excluded sources
	agent = &Agent{
		sources: make(map[string]scraping.Source),
	}

	agent.addSources(sourcesToUse)
	agent.removeSources(sourcesToExclude)

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
		case "otx":
			agent.sources[source] = &otx.Source{}
		case "urlscan":
			agent.sources[source] = &urlscan.Source{}
		case "wayback":
			agent.sources[source] = &wayback.Source{}
		}
	}
}

// removeSources deletes the given sources from the source map
func (agent *Agent) removeSources(sourcesToExclude []string) {
	for _, source := range sourcesToExclude {
		delete(agent.sources, source)
	}
}
