package xurlfind3r

import (
	"context"
	"fmt"
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
	sources           map[string]sources.Source
	includeSubdomains bool
	client            *hqgohttp.Client
}

func (finder *Finder) Find(ctx context.Context, domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	configuration := &sources.Configuration{
		Client: finder.client,
		Extractor: hqgourlextractor.New(
			hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*` + regexp.QuoteMeta(domain) + hqgourlextractor.ExtractorPortOptionalPattern + `)`),
		).CompileRegex(),
		Validator: func(target string) (URL string, valid bool) {
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

			if finder.includeSubdomains {
				pattern = fmt.Sprintf(`https?://([a-z0-9-]+\.)*%s(:\d+)?(?:/[^?\s#]*)?(?:\?[^#\s]*)?(?:#[^\s]*)?`, regexp.QuoteMeta(domain))
			}

			valid = regexp.MustCompile(pattern).MatchString(URL)

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

var (
	Sources = [...]sources.Source{
		arquivo.New(),
		bevigil.New(),
		commoncrawl.New(),
		ghostarchive.New(),
		github.New(),
		gitlab.New(),
		hudsonrock.New(),
		intelx.New(),
		leakradar.New(),
		otx.New(),
		urlscan.New(),
		virustotal.New(),
		wayback.New(),
		zoomeye.New(),
	}
	NameToSourceMap = make(map[string]sources.Source, len(Sources))
)

func init() {
	for i := range Sources {
		source := Sources[i]

		NameToSourceMap[source.Name()] = source
	}
}

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

	if len(cfg.SourcesToUse) < 1 {
		cfg.SourcesToUse = sources.List
	}

	for i := range cfg.SourcesToUse {
		source := cfg.SourcesToUse[i]

		s, k := NameToSourceMap[source]
		if !k {
			continue
		}

		finder.sources[source] = s
	}

	for i := range cfg.SourcesToExclude {
		source := cfg.SourcesToExclude[i]

		delete(finder.sources, source)
	}

	for i := range finder.sources {
		source := finder.sources[i]

		if keys, ok := cfg.Keys[source.Name()]; ok {
			source.UseKeys(keys...)
		}
	}

	return
}
