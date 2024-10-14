package xurlfind3r

import (
	"regexp"
	"sync"

	hqgourl "github.com/hueristiq/hq-go-url"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

// Finder is the main structure that manages the interaction with OSINT sources.
// It holds the available data sources and the configuration used for searching.
type Finder struct {
	// sources is a map of source names to their corresponding implementations.
	// Each source implements the Source interface, which allows domain searches.
	sources map[string]sources.Source
	// configuration contains configuration options such as API keys
	// and other settings needed by the data sources.
	configuration *sources.Configuration
	FilterRegex   *regexp.Regexp
	MatchRegex    *regexp.Regexp
}

func (finder *Finder) IsURLInScope(domain, URL string, subdomainsInScope bool) (URLInScope bool) {
	parsedURL, err := up.Parse(URL)
	if err != nil {
		return
	}

	if parsedURL.Domain == nil {
		return
	}

	ETLDPlusOne := parsedURL.Domain.Root

	if parsedURL.Domain.TopLevel != "" {
		ETLDPlusOne += "." + parsedURL.Domain.TopLevel
	}

	parsedDomain := dp.Parse(domain)

	expectedETLDPlusOne := parsedDomain.Root
	if parsedDomain.TopLevel != "" {
		expectedETLDPlusOne += "." + parsedDomain.TopLevel
	}

	if ETLDPlusOne != expectedETLDPlusOne {
		return
	}

	if !subdomainsInScope &&
		parsedURL.Domain.String() != parsedDomain.String() &&
		parsedURL.Domain.String() != "www."+parsedDomain.String() {

		return
	}

	URLInScope = true

	return
}

// Find takes a domain name and starts the URLs search process across all
// the sources specified in the configuration. It returns a channel through which
// the search results (of type Result) are streamed asynchronously.
func (finder *Finder) Find(domain string) (results chan sources.Result) {
	// Initialize the results channel where URLs findings are sent.
	results = make(chan sources.Result)

	// Parse the given domain using a domain parser.
	parsed := dp.Parse(domain)

	// Rebuild the domain as "root.tld" format.
	domain = parsed.Root + "." + parsed.TopLevel

	finder.configuration.IsInScope = func(URL string) (isInScope bool) {
		return finder.IsURLInScope(domain, URL, finder.configuration.IncludeSubdomains)
	}

	// Launch a goroutine to perform the search concurrently across all sources.
	go func() {
		// Ensure the results channel is closed once all search operations complete.
		defer close(results)

		// A thread-safe map to store already-seen URLs, avoiding duplicates.
		seenURLs := &sync.Map{}

		// WaitGroup ensures all source goroutines finish before exiting.
		wg := &sync.WaitGroup{}

		// Iterate over all the sources in the Finder.
		for name := range finder.sources {
			wg.Add(1)

			// Start a new goroutine for each source to fetch URLs concurrently.
			go func(source sources.Source) {
				// Decrement the WaitGroup counter when this goroutine completes.
				defer wg.Done()

				// Call the source's Run method to start the subdomain search.
				sResults := source.Run(finder.configuration, domain)

				// Process each result as it's received from the source.
				for sResult := range sResults {
					// If the result is a subdomain, process it.
					if sResult.Type == sources.ResultURL {
						// Check if the subdomain has already been seen using sync.Map.
						_, loaded := seenURLs.LoadOrStore(sResult.Value, struct{}{})
						if loaded {
							// If the subdomain is already in the map, skip it.
							continue
						}

						if (finder.MatchRegex != nil && !finder.MatchRegex.MatchString(sResult.Value)) || (finder.FilterRegex != nil && finder.MatchRegex == nil && finder.FilterRegex.MatchString(sResult.Value)) {
							continue
						}
					}

					// Send the result down the results channel.
					results <- sResult
				}
			}(finder.sources[name])
		}

		// Wait for all goroutines to finish before exiting.
		wg.Wait()
	}()

	// Return the channel that will stream URL results.
	return
}

// Configuration holds the configuration for Finder, including
// the sources to use, sources to exclude, and the necessary API keys.
type Configuration struct {
	IncludeSubdomains bool

	// SourcesToUse is a list of source names that should be used for the search.
	SourcesToUse []string
	// SourcesToExclude is a list of source names that should be excluded from the search.
	SourcesToExclude []string
	// Keys contains the API keys for each data source.
	Keys sources.Keys

	FilterPattern string
	MatchPattern  string
}

var (
	// dp is a domain parser used to normalize domains into their root and top-level domain (TLD) components.
	dp = hqgourl.NewDomainParser()
	up = hqgourl.NewParser(hqgourl.ParserWithDefaultScheme("http"))
)

// New creates a new Finder instance based on the provided Configuration.
// It initializes the Finder with the selected sources and ensures that excluded sources are not used.
func New(cfg *Configuration) (finder *Finder, err error) {
	// Initialize a Finder instance with an empty map of sources and the provided configuration.
	finder = &Finder{
		sources: map[string]sources.Source{},
		configuration: &sources.Configuration{
			IncludeSubdomains: cfg.IncludeSubdomains,
			Keys:              cfg.Keys,
		},
	}

	if cfg.FilterPattern != "" {
		finder.FilterRegex, err = regexp.Compile(cfg.FilterPattern)
		if err != nil {
			return
		}
	}

	if cfg.MatchPattern != "" {
		finder.MatchRegex, err = regexp.Compile(cfg.MatchPattern)
		if err != nil {
			return
		}
	}

	// If no specific sources are provided, use the default list of all sources.
	if len(cfg.SourcesToUse) < 1 {
		cfg.SourcesToUse = sources.List
	}

	// Loop through the selected sources and initialize each one
	for _, source := range cfg.SourcesToUse {
		// Depending on the source name, initialize the appropriate source and add it to the map.
		switch source {
		case sources.BEVIGIL:
			finder.sources[source] = &bevigil.Source{}
		case sources.COMMONCRAWL:
			finder.sources[source] = &commoncrawl.Source{}
		case sources.GITHUB:
			finder.sources[source] = &github.Source{}
		case sources.INTELLIGENCEX:
			finder.sources[source] = &intelx.Source{}
		case sources.OPENTHREATEXCHANGE:
			finder.sources[source] = &otx.Source{}
		case sources.URLSCAN:
			finder.sources[source] = &urlscan.Source{}
		case sources.WAYBACK:
			finder.sources[source] = &wayback.Source{}
		}
	}

	// Remove any sources that are specified in the SourcesToExclude list.
	for index := range cfg.SourcesToExclude {
		source := cfg.SourcesToExclude[index]

		delete(finder.sources, source)
	}

	// Return the Finder instance with all the selected sources.
	return
}
