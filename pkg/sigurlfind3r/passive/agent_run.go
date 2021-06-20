package passive

import (
	"regexp"
	"sync"

	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/scraping"
	"github.com/signedsecurity/sigurlfind3r/pkg/sigurlfind3r/session"
)

// Run collects all the known urls for a given domain
func (agent *Agent) Run(domain string, filterRegex *regexp.Regexp, includeSubdomains bool, keys *session.Keys) (URLs chan scraping.URL) {
	URLs = make(chan scraping.URL)

	go func() {
		defer close(URLs)

		ses, err := session.New(domain, filterRegex, includeSubdomains, 10, keys)
		if err != nil {
			return
		}

		wg := &sync.WaitGroup{}

		// Run each source in parallel on the target domain
		for name, source := range agent.sources {
			wg.Add(1)

			go func(name string, source scraping.Source) {
				for res := range source.Run(domain, ses, includeSubdomains) {
					URLs <- res
				}

				wg.Done()
			}(name, source)
		}

		wg.Wait()

	}()

	return
}
