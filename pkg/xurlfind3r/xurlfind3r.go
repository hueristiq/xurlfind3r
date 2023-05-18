package xurlfind3r

import (
	"sync"

	hqurl "github.com/hueristiq/hqgoutils/url"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/commoncrawl"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/github"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/intelx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/otx"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/urlscan"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources/wayback"
)

type Options struct {
	Domain             string
	IncludeSubdomains  bool
	Sources            []string
	Keys               sources.Keys
	ParseWaybackRobots bool
	ParseWaybackSource bool
}

type Finder struct {
	Domain               string
	Sources              map[string]sources.Source
	SourcesConfiguration sources.Configuration
}

func New(options *Options) (finder *Finder) {
	finder = &Finder{
		Domain:  options.Domain,
		Sources: map[string]sources.Source{},
		SourcesConfiguration: sources.Configuration{
			Keys:               options.Keys,
			IncludeSubdomains:  options.IncludeSubdomains,
			ParseWaybackRobots: options.ParseWaybackRobots,
			ParseWaybackSource: options.ParseWaybackSource,
		},
	}

	for index := range options.Sources {
		source := options.Sources[index]

		switch source {
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

	return
}

func (finder *Finder) Find() (URLs chan sources.URL) {
	URLs = make(chan sources.URL)

	go func() {
		defer close(URLs)

		wg := &sync.WaitGroup{}
		seen := &sync.Map{}

		for name := range finder.Sources {
			wg.Add(1)

			source := finder.Sources[name]

			go func(source sources.Source) {
				defer wg.Done()

				for res := range source.Run(finder.SourcesConfiguration, finder.Domain) {
					value := res.Value

					if value == "" {
						continue
					}

					parsedURL, err := hqurl.Parse(value)
					if err != nil {
						continue
					}

					if !finder.SourcesConfiguration.IncludeSubdomains &&
						parsedURL.Host != finder.Domain &&
						parsedURL.Host != "www."+finder.Domain {
						continue
					}

					_, loaded := seen.LoadOrStore(value, struct{}{})
					if loaded {
						continue
					}

					URLs <- res
				}
			}(source)
		}

		wg.Wait()
	}()

	return
}
