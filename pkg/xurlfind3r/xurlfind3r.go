package xurlfind3r

import (
	"context"
	"fmt"
	"hash/fnv"
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

type Finder struct {
	client            *hqgohttp.Client
	sources           map[string]sources.Source
	includeSubdomains bool
}

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

		deduper := newDeduper()

		wg := &sync.WaitGroup{}

		for name := range finder.sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				sResults := source.Run(ctx, configuration, domain)

				for sResult := range sResults {
					if sResult.Type == sources.ResultURL {
						if deduper.seen(sResult.Value) {
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

func hostPattern(domain string) (pattern string) {
	pattern = `(?i)(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.PortOptionalPattern + `)`

	return
}

func compileExtractor(domain string) (extractor *regexp.Regexp, err error) {
	extractor, err = hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(hostPattern(domain)),
	).CompileRegex()
	if err != nil {
		return nil, fmt.Errorf("failed to compile URL extractor: %w", err)
	}

	return
}

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

func isASCIIAlpha(character byte) (ok bool) {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

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

type ClientConfiguration struct {
	UserAgent string
}

type Configuration struct {
	Client            *ClientConfiguration
	SourcesToUse      []string
	SourcesToExclude  []string
	Keys              map[string]sources.Keys
	IncludeSubdomains bool
}

var registry = map[string]func() sources.Source{
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
	constructor, exists := registry[name]
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
		sources:           make(map[string]sources.Source, len(sources.List)),
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
			err = fmt.Errorf("unknown source %q; supported sources: %s", name, strings.Join(sources.List, ", "))

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

const dedupShardCount = 32

type deduper struct {
	shards [dedupShardCount]dedupShard
}

type dedupShard struct {
	mu      sync.Mutex
	entries map[uint64]struct{}
}

func newDeduper() (d *deduper) {
	d = &deduper{}

	for i := range d.shards {
		d.shards[i].entries = make(map[uint64]struct{})
	}

	return
}

func (d *deduper) seen(value string) (duplicate bool) {
	hash := dedupHash(value)

	shard := &d.shards[hash%dedupShardCount]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.entries[hash]; exists {
		return true
	}

	shard.entries[hash] = struct{}{}

	return false
}

func dedupHash(value string) (hash uint64) {
	hasher := fnv.New64a()

	_, _ = hasher.Write([]byte(value))

	hash = hasher.Sum64()

	return
}
