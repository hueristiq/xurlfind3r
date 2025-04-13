// Package xurlfind3r provides the core functionality for performing URL
// discovery using multiple data sources. It integrates various sources that implement
// the sources.Source interface, coordinates concurrent URL enumeration, and
// aggregates the results.
//
// The package defines a Finder type, which manages enabled sources and configuration
// settings, and provides a Find method to initiate URL discovery for a given domain.
// It also defines a Configuration type for user-defined settings and API keys, and
// initializes HTTP client configurations for reliable network requests.
package xurlfind3r

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgourlextractor "github.com/hueristiq/hq-go-url/extractor"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/virustotal"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

// Finder is the primary structure for performing URL discovery.
// It manages data sources and configuration settings.
//
// Fields:
//   - sources (map[string]sources.Source): A map of string keys to sources.Source interfaces representing the enabled enumeration sources.
//   - configuration (*sources.Configuration): A pointer to the sources.Configuration struct containing API keys and other settings.
type Finder struct {
	sources       map[string]sources.Source
	configuration *sources.Configuration
}

// Find initiates the URL discovery process for a specific domain.
// It normalizes the domain name, applies source-specific logic, and streams results via a channel.
// The method uses all enabled sources concurrently and aggregates their results.
//
// Parameters:
//   - domain (string): The target domain for URL discovery.
//
// Returns:
//   - results (chan sources.Result): A channel that streams URL enumeration results.
func (finder *Finder) Find(domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	finder.configuration.Extractor = hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.ExtractorPortOptionalPattern + `)`),
	).CompileRegex()

	finder.configuration.Validate = func(target string) (URL string, valid bool) {
		scheme := "https"

		switch {
		case strings.HasPrefix(target, "//"):
			URL = scheme + ":" + target
		case strings.HasPrefix(target, "://"):
			URL = scheme + target
		case !strings.Contains(target, "//"):
			URL = scheme + "://" + target
		default:
			URL = target
		}

		pattern := fmt.Sprintf(`https?://(www\.)?%s(:\d+)?(?:/[^?\s#]*)?(?:\?[^#\s]*)?(?:#[^\s]*)?`, regexp.QuoteMeta(domain))

		if finder.configuration.IncludeSubdomains {
			pattern = fmt.Sprintf(`https?://([a-z0-9-]+\.)*%s(:\d+)?(?:/[^?\s#]*)?(?:\?[^#\s]*)?(?:#[^\s]*)?`, regexp.QuoteMeta(domain))
		}

		valid = regexp.MustCompile(pattern).MatchString(URL)

		return
	}

	go func() {
		defer close(results)

		seenURLs := &sync.Map{}

		wg := &sync.WaitGroup{}

		for name := range finder.sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				sResults := source.Run(domain, finder.configuration)

				for sResult := range sResults {
					if sResult.Type == sources.ResultURL {
						_, loaded := seenURLs.LoadOrStore(sResult.Value, struct{}{})
						if loaded {
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
// - SourcesToUSe ([]string): List of source names to be used for enumeration.
// - SourcesToExclude ([]string): List of source names to be excluded from enumeration.
// - Keys (sources.Keys): API keys for authenticated sources.
type Configuration struct {
	IncludeSubdomains bool
	SourcesToUse      []string
	SourcesToExclude  []string
	Keys              sources.Keys
}

func init() {
	cfg := hqgohttp.DefaultSprayingClientConfiguration

	cfg.Timeout = 1 * time.Hour

	hqgohttp.DefaultClient, _ = hqgohttp.NewClient(cfg)
}

// New initializes a new Finder instance with the specified configuration.
// It sets up the enabled sources, applies exclusions, and configures the Finder.
//
// Parameters:
//   - cfg (*Configuration): The user-defined configuration for sources and API keys.
//
// Returns:
//   - finder (*Finder): A pointer to the initialized Finder instance.
//   - err (error): An error object if initialization fails, or nil on success.
func New(cfg *Configuration) (finder *Finder, err error) {
	finder = &Finder{
		sources: map[string]sources.Source{},
		configuration: &sources.Configuration{
			IncludeSubdomains: cfg.IncludeSubdomains,
			Keys:              cfg.Keys,
		},
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
		case sources.VIRUSTOTAL:
			finder.sources[source] = &virustotal.Source{}
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
