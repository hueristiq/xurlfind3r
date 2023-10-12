package scraper

import (
	"regexp"
	"sync"

	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/bevigil"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/scraper/sources/wayback"
)

type Options struct {
	IncludeSubdomains  bool
	SourcesToUSe       []string
	SourcesToExclude   []string
	Keys               sources.Keys
	ParseWaybackRobots bool
	ParseWaybackSource bool
	FilterPattern      string
	Matchattern        string
}

type Finder struct {
	Sources              map[string]sources.Source
	SourcesConfiguration *sources.Configuration
	FilterRegex          *regexp.Regexp
	MatchRegex           *regexp.Regexp
}

func (finder *Finder) Scrape(domain string) (results chan sources.Result) {
	results = make(chan sources.Result)

	go func() {
		defer close(results)

		seenURLs := &sync.Map{}

		wg := &sync.WaitGroup{}

		for name := range finder.Sources {
			wg.Add(1)

			go func(source sources.Source) {
				defer wg.Done()

				sResults := source.Run(finder.SourcesConfiguration, domain)

				for sResult := range sResults {
					if sResult.Type == sources.URL {
						_, loaded := seenURLs.LoadOrStore(sResult.Value, struct{}{})
						if loaded {
							continue
						}

						if finder.MatchRegex != nil {
							if !finder.MatchRegex.MatchString(sResult.Value) {
								continue
							}
						} else if finder.FilterRegex != nil && finder.MatchRegex == nil {
							if finder.FilterRegex.MatchString(sResult.Value) {
								continue
							}
						}
					}

					results <- sResult
				}
			}(finder.Sources[name])
		}

		wg.Wait()
	}()

	return
}

func New(options *Options) (finder *Finder, err error) {
	finder = &Finder{
		Sources: map[string]sources.Source{},
		SourcesConfiguration: &sources.Configuration{
			IncludeSubdomains:  options.IncludeSubdomains,
			Keys:               options.Keys,
			ParseWaybackRobots: options.ParseWaybackRobots,
			ParseWaybackSource: options.ParseWaybackSource,
		},
	}

	if options.FilterPattern != "" {
		finder.FilterRegex, err = regexp.Compile(options.FilterPattern)
		if err != nil {
			return
		}
	}

	if options.Matchattern != "" {
		finder.MatchRegex, err = regexp.Compile(options.Matchattern)
		if err != nil {
			return
		}
	}

	// Sources To Use
	if len(options.SourcesToUSe) < 1 {
		options.SourcesToUSe = sources.List
	}

	for index := range options.SourcesToUSe {
		source := options.SourcesToUSe[index]

		switch source {
		case "bevigil":
			finder.Sources[source] = &bevigil.Source{}
		case "commoncrawl":
			finder.Sources[source] = &commoncrawl.Source{}
		case "github":
			finder.Sources[source] = &github.Source{}
		case "intelx":
			finder.Sources[source] = &intelx.Source{}
		case "otx":
			finder.Sources[source] = &otx.Source{}
		case "urlscan":
			finder.Sources[source] = &urlscan.Source{}
		case "wayback":
			finder.Sources[source] = &wayback.Source{}
		}
	}

	// Sources To Exclude
	for index := range options.SourcesToExclude {
		source := options.SourcesToExclude[index]

		delete(finder.Sources, source)
	}

	return
}
