package xurlfind3r

import (
	"regexp"
	"sync"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
	hqgourl "go.source.hueristiq.com/url"
)

// Finder is the primary structure for performing URL discovery.
// It manages data sources and configuration settings.
//
// Fields:
// - sources: A map of enabled data sources, keyed by their names.
// - configuration: Holds settings like API keys, filtering patterns, and inclusion of subdomains.
// - FilterRegex: Regular expression to filter out specific URLs.
// - MatchRegex: Regular expression to match specific URLs.
type Finder struct {
	sources       map[string]sources.Source
	configuration *sources.Configuration
	FilterRegex   *regexp.Regexp
	MatchRegex    *regexp.Regexp
}

// IsURLInScope determines if a given URL is within the scope of a target domain.
//
// Parameters:
// - domain string: The target domain.
// - URL string: The URL to check.
// - subdomainsInScope bool: Whether subdomains should be considered within scope.
//
// Returns:
// - URLInScope bool: True if the URL is in scope, otherwise false.
func (finder *Finder) IsURLInScope(domain, URL string, subdomainsInScope bool) (URLInScope bool) {
	parsedURL, err := up.Parse(URL)
	if err != nil {
		return
	}

	if parsedURL.Domain == nil {
		return
	}

	ETLDPlusOne := parsedURL.Domain.SLD

	if parsedURL.Domain.TLD != "" {
		ETLDPlusOne += "." + parsedURL.Domain.TLD
	}

	parsedDomain := dp.Parse(domain)

	expectedETLDPlusOne := parsedDomain.SLD
	if parsedDomain.TLD != "" {
		expectedETLDPlusOne += "." + parsedDomain.TLD
	}

	if ETLDPlusOne != expectedETLDPlusOne {
		return
	}

	if !subdomainsInScope && parsedURL.Domain.String() != parsedDomain.String() && parsedURL.Domain.String() != "www."+parsedDomain.String() {
		return
	}

	URLInScope = true

	return
}

// Find performs URLs discovery for a given domain.
// It uses all the enabled sources and streams the results asynchronously through a channel.
//
// Parameters:
// - domain string: The target domain to find URLs for.
//
// Returns:
// - results chan sources.Result: A channel that streams the results of type `sources.Result`.
func (finder *Finder) Find(domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	parsed := dp.Parse(domain)

	domain = parsed.SLD + "." + parsed.TLD

	finder.configuration.IsInScope = func(URL string) (isInScope bool) {
		return finder.IsURLInScope(domain, URL, finder.configuration.IncludeSubdomains)
	}

	go func() {
		defer close(results)

		seenURLs := &sync.Map{}

		wg := &sync.WaitGroup{}

		for name := range finder.sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				sResults := source.Run(finder.configuration, domain)

				for sResult := range sResults {
					if sResult.Type == sources.ResultURL {
						_, loaded := seenURLs.LoadOrStore(sResult.Value, struct{}{})
						if loaded {
							continue
						}

						if (finder.MatchRegex != nil && !finder.MatchRegex.MatchString(sResult.Value)) || (finder.FilterRegex != nil && finder.MatchRegex == nil && finder.FilterRegex.MatchString(sResult.Value)) {
							continue
						}
					}

					results <- sResult
				}
			}(finder.sources[name])
		}

		wg.Wait()
	}()

	return
}

// Configuration represents the user-defined settings for the Finder.
// It specifies which sources to use or exclude and includes API keys for external sources.
//
// Fields:
// - IncludeSubdomains bool: Whether to include subdomains in the scope.
// - SourcesToUse []string: A list of sources to enable.
// - SourcesToExclude []string: A list of sources to exclude.
// - Keys sources.Keys: API keys for various sources.
// - FilterPattern string: Regular expression for filtering URLs.
// - MatchPattern string: Regular expression for matching URLs.
type Configuration struct {
	IncludeSubdomains bool
	SourcesToUse      []string
	SourcesToExclude  []string
	Keys              sources.Keys
	FilterPattern     string
	MatchPattern      string
}

var (
	// dp is a domain parser used to extract root and top-level domains.
	dp = hqgourl.NewDomainParser()
	// up is a URL parser initialized with a default scheme of "http".
	up = hqgourl.NewParser(hqgourl.ParserWithDefaultScheme("http"))
)

// New creates and initializes a new Finder instance.
// It enables the specified sources, applies exclusions, and sets the required configuration.
//
// Parameters:
// - cfg *Configuration: The configuration specifying sources, exclusions, and API keys.
//
// Returns:
// - finder *Finder: A pointer to the initialized Finder instance.
// - err error: Returns an error if initialization fails, otherwise nil.
func New(cfg *Configuration) (finder *Finder, err error) {
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

	if len(cfg.SourcesToUse) < 1 {
		cfg.SourcesToUse = sources.List
	}

	for _, source := range cfg.SourcesToUse {
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

	for index := range cfg.SourcesToExclude {
		source := cfg.SourcesToExclude[index]

		delete(finder.sources, source)
	}

	return
}
