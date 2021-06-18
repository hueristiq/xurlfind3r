package passive

import (
	"sync"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/commoncrawl"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/github"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/otx"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/urlscan"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping/sources/wayback"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

type Agent struct {
	sources map[string]scraping.Source
}

// New creates a new agent for passive subdomain discovery
func New(sourcesToUse, sourcesToExclude []string) (agent *Agent) {
	// Create the agent, insert the sources and remove the excluded sources
	agent = &Agent{sources: make(map[string]scraping.Source)}

	agent = &Agent{
		sources: make(map[string]scraping.Source),
	}

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

	for _, source := range sourcesToExclude {
		delete(agent.sources, source)
	}

	return
}

// EnumerateSubdomains enumerates all the subdomains for a given domain
func (agent *Agent) Enumerate(domain string, subs bool, keys *session.Keys) (URLs chan scraping.URL) {
	URLs = make(chan scraping.URL)

	go func() {
		defer close(URLs)

		ses, err := session.New(domain, subs, 10, keys)
		if err != nil {
			return
		}

		wg := &sync.WaitGroup{}

		// Run each source in parallel on the target domain
		for name, source := range agent.sources {
			wg.Add(1)

			go func(name string, source scraping.Source) {
				for res := range source.Run(domain, ses, subs) {
					URLs <- res
				}

				wg.Done()
			}(name, source)
		}

		wg.Wait()

	}()

	return
}
