package xurlfind3r

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/arquivo"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/ghostarchive"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/gitlab"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/hudsonrock"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/leakradar"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/virustotal"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/zoomeye"
)

// Finder discovers URLs for a domain across the configured sources.
//
// Construct one with [New]; a Finder is safe for concurrent use.
//
// Fields:
//   - sources (map[string]sources.Source): the sources to query, by name.
//   - includeSubdomains (bool): whether subdomains are in scope.
//   - client (*hqgohttp.Client): the shared HTTP client.
type Finder struct {
	sources           map[string]sources.Source
	includeSubdomains bool
	client            *hqgohttp.Client
}

// Find queries every configured source for URLs belonging to domain
// and streams the deduplicated results.
//
// Parameters:
//   - ctx (context.Context): the run context.
//   - domain (string): the target domain.
//
// Returns:
//   - results (chan sources.Result): the result stream, closed when
//     all sources finish.
func (finder *Finder) Find(ctx context.Context, domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	extractor, err := compileExtractor(domain)
	if err != nil {
		result := sources.Result{
			Type:   sources.ResultError,
			Source: "xurlfind3r",
			Error:  err,
		}

		results <- result

		close(results)

		return results
	}

	configuration := &sources.Configuration{
		Client:    finder.client,
		Extractor: extractor,
		Validator: func(target string) (URL string, valid bool) {
			URL, valid = validateURL(target, domain, finder.includeSubdomains)

			return
		},
	}

	go func() {
		defer close(results)

		seenURLs := &sync.Map{}

		wg := &sync.WaitGroup{}

		for name := range finder.sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				sResults := source.Run(ctx, configuration, domain)

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

// hostPattern returns the extractor host pattern scoped to domain.
//
// The pattern is case-insensitive so that uppercase hosts (e.g.
// EXAMPLE.COM) are extracted; the trailing host-boundary enforcement
// happens in [sources.ExtractURLs], where the match context is known.
//
// Parameters:
//   - domain (string): the target domain.
//
// Returns:
//   - pattern (string): the host pattern for the extractor.
func hostPattern(domain string) (pattern string) {
	pattern = `(?i)(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.PortOptionalPattern + `)`

	return
}

// compileExtractor compiles the URL extractor scoped to domain.
//
// Parameters:
//   - domain (string): the target domain.
//
// Returns:
//   - extractor (*regexp.Regexp): the compiled extractor regular expression.
//   - err (error): non-nil if the extractor pattern fails to compile.
func compileExtractor(domain string) (extractor *regexp.Regexp, err error) {
	extractor, err = hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(hostPattern(domain)),
	).CompileRegex()
	if err != nil {
		return nil, fmt.Errorf("failed to compile URL extractor: %w", err)
	}

	return
}

// hasScheme reports whether the target starts with a URI scheme
// (e.g. "mailto:", "https:").
//
// Parameters:
//   - target (string): the candidate URL.
//
// Returns:
//   - ok (bool): true if target starts with a scheme.
func hasScheme(target string) (ok bool) {
	if target == "" {
		return false
	}

	if !isASCIIAlpha(target[0]) {
		return false
	}

	for index := 1; index < len(target); index++ {
		character := target[index]

		switch {
		case character == ':':
			return true
		case isASCIIAlpha(character), character >= '0' && character <= '9', character == '+', character == '-', character == '.':
			continue
		default:
			return false
		}
	}

	return false
}

// isASCIIAlpha reports whether the character is an ASCII letter.
func isASCIIAlpha(character byte) (ok bool) {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

// validateURL normalizes the candidate target URL and checks that its
// host belongs to domain's scope.
//
// Parameters:
//   - target (string): the candidate URL, as extracted from a source.
//   - domain (string): the target domain defining the scope.
//   - includeSubdomains (bool): whether subdomains of domain are in scope.
//
// Returns:
//   - URL (string): the normalized URL.
//   - valid (bool): true if the URL is http(s), its host is domain,
//     www.domain or, when includeSubdomains is set, a subdomain of domain.
func validateURL(target, domain string, includeSubdomains bool) (URL string, valid bool) {
	scheme := "https"

	switch {
	case strings.HasPrefix(target, "//"):
		URL = scheme + ":" + target
	case strings.HasPrefix(target, "://"):
		URL = scheme + target
	case hasScheme(target):
		URL = target
	case !strings.Contains(target, "//"):
		URL = scheme + "://" + target
	default:
		URL = target
	}

	parsed, err := url.Parse(URL)
	if err != nil {
		return URL, false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return URL, false
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return URL, false
	}

	domain = strings.ToLower(domain)

	if includeSubdomains {
		valid = hostname == domain || strings.HasSuffix(hostname, "."+domain)

		return URL, valid
	}

	valid = hostname == domain || hostname == "www."+domain

	return URL, valid
}

// ClientConfiguration configures the finder's HTTP client.
//
// Fields:
//   - UserAgent (string): the User-Agent header; empty selects the
//     client default.
type ClientConfiguration struct {
	UserAgent string
}

// Configuration configures a [Finder].
//
// Fields:
//   - Client (*ClientConfiguration): HTTP client settings.
//   - SourcesToUse ([]string): source names to use; empty selects all.
//   - SourcesToExclude ([]string): source names to skip.
//   - Keys (map[string]sources.Keys): per-source API keys.
//   - IncludeSubdomains (bool): whether subdomains are in scope.
type Configuration struct {
	Client            *ClientConfiguration
	SourcesToUse      []string
	SourcesToExclude  []string
	Keys              map[string]sources.Keys
	IncludeSubdomains bool
}

// Sources lists the registered source names in canonical order.
var Sources = []string{
	sources.Arquivo,
	sources.Bevigil,
	sources.Commoncrawl,
	sources.Ghostarchive,
	sources.Github,
	sources.Gitlab,
	sources.Hudsonrock,
	sources.IntelligenceX,
	sources.Leakradar,
	sources.OpenThreatExchange,
	sources.URLScan,
	sources.Virustotal,
	sources.Wayback,
	sources.Zoomeye,
}

// sourceRegistry maps source names to their constructors. It is written
// once at package load and only read afterwards, so constructors are
// safe for concurrent use.
var sourceRegistry = map[string]func() sources.Source{
	sources.Arquivo:            arquivo.New,
	sources.Bevigil:            bevigil.New,
	sources.Commoncrawl:        commoncrawl.New,
	sources.Ghostarchive:       ghostarchive.New,
	sources.Github:             github.New,
	sources.Gitlab:             gitlab.New,
	sources.Hudsonrock:         hudsonrock.New,
	sources.IntelligenceX:      intelx.New,
	sources.Leakradar:          leakradar.New,
	sources.OpenThreatExchange: otx.New,
	sources.URLScan:            urlscan.New,
	sources.Virustotal:         virustotal.New,
	sources.Wayback:            wayback.New,
	sources.Zoomeye:            zoomeye.New,
}

// NewSource returns a fresh instance of the source registered under
// name.
//
// Parameters:
//   - name (string): the registered source name.
//
// Returns:
//   - source (sources.Source): the new source instance; nil when name
//     is unknown.
//   - ok (bool): whether name is registered.
func NewSource(name string) (source sources.Source, ok bool) {
	constructor, exists := sourceRegistry[name]
	if !exists {
		return nil, false
	}

	return constructor(), true
}

// New returns a finder configured with cfg. Every finder owns fresh
// source instances, so finders are safe to construct and use
// concurrently.
//
// Parameters:
//   - cfg (*Configuration): the finder configuration.
//
// Returns:
//   - finder (*Finder): the configured finder.
//   - err (error): non-nil when the HTTP client cannot be initialized
//     or a requested source name is unknown.
func New(cfg *Configuration) (finder *Finder, err error) {
	finder = &Finder{
		sources:           make(map[string]sources.Source, len(Sources)),
		includeSubdomains: cfg.IncludeSubdomains,
	}

	ofs := []hqgohttp.ClientOptionFunc{
		hqgohttp.WithHTTPClient(hqgohttp.DefaultHTTPClient()),
		hqgohttp.WithTimeout(1 * time.Hour),
		hqgohttp.WithMaxRetries(3),
		hqgohttp.WithRetryWaitMin(1 * time.Second),
		hqgohttp.WithRetryWaitMax(30 * time.Second),
		hqgohttp.WithResponseReadLimit(4096),
	}

	if cfg.Client != nil && cfg.Client.UserAgent != "" {
		ofs = append(ofs, hqgohttp.WithUserAgent(cfg.Client.UserAgent))
	}

	finder.client, err = hqgohttp.NewClient(ofs...)
	if err != nil {
		err = fmt.Errorf("failed to initialize HTTP client: %w", err)

		return
	}

	sourcesToUse := cfg.SourcesToUse
	if len(sourcesToUse) < 1 {
		sourcesToUse = sources.List
	}

	for i := range sourcesToUse {
		name := sourcesToUse[i]

		source, ok := NewSource(name)
		if !ok {
			err = fmt.Errorf("unknown source %q; supported sources: %s", name, strings.Join(Sources, ", "))

			return
		}

		finder.sources[name] = source
	}

	for i := range cfg.SourcesToExclude {
		delete(finder.sources, cfg.SourcesToExclude[i])
	}

	for i := range finder.sources {
		source := finder.sources[i]

		if keys, ok := cfg.Keys[source.Name()]; ok {
			source.UseKeys(keys...)
		}
	}

	return
}
