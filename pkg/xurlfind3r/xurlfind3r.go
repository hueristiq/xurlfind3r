package xurlfind3r

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"html"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	hqgohttp "github.com/hueristiq/hq-lib-http-go"
	hqgohttpheader "github.com/hueristiq/hq-lib-http-go/header"
	hqgoretrier "github.com/hueristiq/hq-lib-retrier-go"
	hqgoretrierbackoff "github.com/hueristiq/hq-lib-retrier-go/backoff"
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
	results = make(chan sources.Result, 1)

	domain = strings.ToLower(domain)

	extractor, err := compileExtractor(domain)
	if err != nil {
		results <- sources.ErrorResult("xurlfind3r", err)

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

		for _, source := range finder.sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				relayResults(ctx, source.Run(ctx, configuration, domain), deduper, results)
			}(source)
		}

		wg.Wait()
	}()

	return
}

func relayResults(ctx context.Context, sourceResults <-chan sources.Result, deduper *deduper, results chan<- sources.Result) {
	for {
		select {
		case <-ctx.Done():
			drainResults(sourceResults)

			return
		case result, ok := <-sourceResults:
			if !ok {
				return
			}

			if result.Type == sources.ResultURL && deduper.seen(result.Value) {
				continue
			}

			select {
			case results <- result:
			case <-ctx.Done():
				drainResults(sourceResults)

				return
			}
		}
	}
}

func drainResults(sourceResults <-chan sources.Result) {
	for range sourceResults {
	}
}

type deduper struct {
	shards [dedupShardCount]dedupShard
}

func (d *deduper) seen(value string) (duplicate bool) {
	key := canonicalURLKey(value)
	hash := dedupHash(key)

	shard := &d.shards[hash%dedupShardCount]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.entries[key]; exists {
		return true
	}

	shard.entries[key] = struct{}{}

	return false
}

func canonicalURLKey(value string) (key string) {
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}

	scheme := strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()

	if scheme == schemeHTTP && port == "80" || scheme == schemeHTTPS && port == "443" {
		port = ""
	}

	host := hostname

	switch {
	case port != "":
		host = net.JoinHostPort(hostname, port)
	case strings.Contains(hostname, ":"):
		host = "[" + hostname + "]"
	}

	parsed.Scheme = scheme
	parsed.Host = host
	parsed.Fragment = ""
	parsed.RawFragment = ""

	if parsed.Path == "" {
		parsed.Path = "/"
		parsed.RawPath = ""
	}

	parsed.RawQuery = parsed.Query().Encode()

	key = parsed.String()

	return
}

type dedupShard struct {
	mu      sync.Mutex
	entries map[string]struct{}
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
	Timeout           time.Duration
}

const (
	dedupShardCount = 32

	defaultTimeout = 30 * time.Second

	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

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

func New(cfg *Configuration) (finder *Finder, err error) {
	if cfg == nil {
		err = errors.New("configuration is nil")

		return
	}

	finder = &Finder{
		sources:           make(map[string]sources.Source, len(sources.List)),
		includeSubdomains: cfg.IncludeSubdomains,
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	ofs := []hqgohttp.ClientOptionFunc{
		hqgohttp.WithHTTPClient(hqgohttp.DefaultHTTPPooledClient()),
		hqgohttp.WithTimeout(timeout),
		hqgohttp.WithRetrier(newRetrier()),
		hqgohttp.WithResponseReadLimit(4096),
	}

	if cfg.Client != nil && cfg.Client.UserAgent != "" {
		ofs = append(ofs, hqgohttp.WithHeader(hqgohttpheader.UserAgent.String(), cfg.Client.UserAgent))
	}

	finder.client = hqgohttp.New(ofs...)

	sourcesToUse := cfg.SourcesToUse

	if len(sourcesToUse) < 1 {
		sourcesToUse = sources.List
	}

	for i := range sourcesToUse {
		name := sourcesToUse[i]

		source, ok := NewSource(name)
		if !ok {
			err = fmt.Errorf("unknown source %q", name)

			return
		}

		finder.sources[name] = source
	}

	for _, name := range cfg.SourcesToExclude {
		if _, ok := registry[name]; !ok {
			err = fmt.Errorf("unknown source %q in sources to exclude", name)

			return
		}

		delete(finder.sources, name)
	}

	if len(finder.sources) == 0 {
		err = errors.New("no sources left after applying exclusions")

		return
	}

	for _, source := range finder.sources {
		if keys, ok := cfg.Keys[source.Name()]; ok {
			source.UseKeys(keys...)
		}
	}

	return
}

func (finder *Finder) SourceCount() (count int) {
	count = len(finder.sources)

	return
}

func NewSource(name string) (source sources.Source, ok bool) {
	constructor, exists := registry[name]
	if !exists {
		return nil, false
	}

	return constructor(), true
}

func compileExtractor(domain string) (extractor *regexp.Regexp, err error) {
	extractor, err = hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(hostPattern(domain)),
		hqgourlextractor.WithoutEmail(),
		hqgourlextractor.WithoutUserInfo(),
	).CompileRegex()
	if err != nil {
		return nil, fmt.Errorf("failed to compile URL extractor: %w", err)
	}

	return
}

func hostPattern(domain string) (pattern string) {
	label := `[a-z0-9_](?:[a-z0-9_-]*[a-z0-9_])?`

	pattern = `(?i)(?:(?:` + label + `[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.PortOptionalPattern + `)`

	return
}

func validateURL(target, domain string, includeSubdomains bool) (URL string, valid bool) {
	URL = strings.Trim(target, `'"`)
	URL = html.UnescapeString(URL)

	if hasInvalidURLCharacters(URL) {
		return URL, false
	}

	switch {
	case strings.HasPrefix(URL, "//"):
		URL = "https:" + URL
	case strings.HasPrefix(URL, "://"):
		URL = "https://" + strings.TrimPrefix(URL, "://")
	}

	parsed, err := url.Parse(URL)
	if err != nil {
		return URL, false
	}

	if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS || parsed.Hostname() == "" {
		URL = "https://" + URL

		if parsed, err = url.Parse(URL); err != nil {
			return URL, false
		}
	}

	if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
		return URL, false
	}

	if parsed.User != nil {
		return URL, false
	}

	domain = strings.ToLower(domain)

	hostname := strings.ToLower(parsed.Hostname())

	if hostname == "" {
		return URL, false
	}

	if !hostnameMatches(hostname, domain, includeSubdomains) {
		return URL, false
	}

	normalizedHost := hostname

	if port := parsed.Port(); port != "" {
		normalizedHost += ":" + port
	}

	if normalizedHost != parsed.Host {
		URL = strings.Replace(URL, parsed.Host, normalizedHost, 1)
	}

	URL = strings.TrimRight(URL, "'")

	return URL, true
}

// hasInvalidURLCharacters reports whether rawURL contains characters that
// RFC 3986 requires to be percent-encoded.
func hasInvalidURLCharacters(rawURL string) (invalid bool) {
	return strings.IndexFunc(rawURL, func(r rune) bool {
		return r == '"' || r == '\\' || r == '<' || r == '>' || unicode.IsControl(r) || unicode.IsSpace(r)
	}) >= 0
}

func hostnameMatches(hostname, domain string, includeSubdomains bool) (matches bool) {
	if includeSubdomains {
		matches = hostname == domain || strings.HasSuffix(hostname, "."+domain)

		return
	}

	matches = hostname == domain || hostname == "www."+domain

	return
}

func newDeduper() (d *deduper) {
	d = &deduper{}

	for i := range d.shards {
		d.shards[i].entries = make(map[string]struct{})
	}

	return
}

func dedupHash(value string) (hash uint64) {
	hasher := fnv.New64a()

	_, _ = hasher.Write([]byte(value))

	hash = hasher.Sum64()

	return
}

func newRetrier() (retrier hqgohttp.Retrier) {
	retrier = hqgohttp.RetrierFunc(func(ctx context.Context, operation hqgohttp.RetryOperation, options hqgohttp.RetrierOptions) (*http.Response, error) {
		return hqgoretrier.RetryWithData(ctx, operation,
			hqgoretrier.WithMaxAttempts(4),
			hqgoretrier.WithWaitMin(1*time.Second),
			hqgoretrier.WithWaitMax(30*time.Second),
			hqgoretrier.WithBackoff(func(minDelay, maxDelay time.Duration) hqgoretrierbackoff.Backoff {
				base := hqgoretrierbackoff.ExponentialWithDecorrelatedJitter(minDelay, maxDelay)

				return func(attempt int) time.Duration {
					if wait, ok := options.RetryAfter(); ok {
						return min(wait, maxDelay)
					}

					return base(attempt)
				}
			}),
			hqgoretrier.WithRetryOn(options.RetryOn),
			hqgoretrier.WithOnRetry(func(event hqgoretrier.RetryEvent) {
				options.OnRetry(hqgohttp.RetrierEvent{Attempt: event.Attempt, Err: event.Err, Wait: event.Wait})
			}),
		)
	})

	return
}
