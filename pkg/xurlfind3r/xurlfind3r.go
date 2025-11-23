package xurlfind3r

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	hqgohttpheader "github.com/hueristiq/hq-go-http/header"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/hudsonrock"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/leakradar"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/virustotal"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

type Finder struct {
	sources            map[string]sources.Source
	includceSubdomains bool
}

func (finder *Finder) Find(domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	configuration := &sources.Configuration{
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

			if finder.includceSubdomains {
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

				sResults := source.Run(configuration, domain)

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
		bevigil.New(),
		commoncrawl.New(),
		github.New(),
		hudsonrock.New(),
		intelx.New(),
		leakradar.New(),
		otx.New(),
		urlscan.New(),
		virustotal.New(),
		wayback.New(),
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
		sources:            make(map[string]sources.Source, len(Sources)),
		includceSubdomains: cfg.IncludeSubdomains,
	}

	cc := hqgohttp.DefaultSprayingClientConfiguration

	cc.Headers = []hqgohttp.Header{}
	cc.Timeout = 1 * time.Hour

	if cfg.Client != nil && cfg.Client.UserAgent != "" {
		cc.Headers = append(cc.Headers, hqgohttp.NewSetHeader(hqgohttpheader.UserAgent.String(), cfg.Client.UserAgent))
	}

	hqgohttp.DefaultClient, err = hqgohttp.NewClient(cc)
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
