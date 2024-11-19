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
	sources       map[string]sources.Source
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

// Configuration holds the configuration for Finder, including
// the sources to use, sources to exclude, and the necessary API keys.
type Configuration struct {
	IncludeSubdomains bool
	SourcesToUse      []string
	SourcesToExclude  []string
	Keys              sources.Keys
	FilterPattern     string
	MatchPattern      string
}

var (
	// dp is a domain parser used to normalize domains into their root and top-level domain (TLD) components.
	dp = hqgourl.NewDomainParser()
	up = hqgourl.NewParser(hqgourl.ParserWithDefaultScheme("http"))
)

// New creates a new Finder instance based on the provided Configuration.
// It initializes the Finder with the selected sources and ensures that excluded sources are not used.
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
