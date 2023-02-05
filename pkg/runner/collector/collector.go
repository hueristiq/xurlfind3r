package collector

import (
	"sync"

	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/filter"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/output"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/commoncrawl"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/github"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/intelx"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/otx"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/urlscan"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/wayback"
	"github.com/hueristiq/hqurlfind3r/pkg/runner/collector/sources/waybackrobots"
)

type Collector struct {
	sources map[string]sources.Source
	keys    sources.Keys
	filter  filter.Filter
}

func New(sourcesToUse, sourcesToExclude []string, keys sources.Keys, filter filter.Filter) (collector *Collector) {
	if len(sourcesToUse) <= 0 {
		sourcesToUse = append(sourcesToUse, sources.List...)
	}

	collector = &Collector{
		sources: make(map[string]sources.Source),
		keys:    keys,
		filter:  filter,
	}

	collector.addSources(sourcesToUse)
	collector.removeSources(sourcesToExclude)

	return
}

func (collector *Collector) addSources(sourcesToUse []string) {
	for index := range sourcesToUse {
		source := sourcesToUse[index]

		switch source {
		case "commoncrawl":
			collector.sources[source] = &commoncrawl.Source{}
		case "github":
			collector.sources[source] = &github.Source{}
		case "intelx":
			collector.sources[source] = &intelx.Source{}
		case "otx":
			collector.sources[source] = &otx.Source{}
		case "urlscan":
			collector.sources[source] = &urlscan.Source{}
		case "wayback":
			collector.sources[source] = &wayback.Source{}
		case "waybackrobots":
			collector.sources[source] = &waybackrobots.Source{}
		}
	}
}

func (collector *Collector) removeSources(sourcesToExclude []string) {
	for index := range sourcesToExclude {
		source := sourcesToExclude[index]

		delete(collector.sources, source)
	}
}

func (collector *Collector) Collect() (URLs chan output.URL) {
	URLs = make(chan output.URL)

	go func() {
		defer close(URLs)

		wg := &sync.WaitGroup{}

		for name, source := range collector.sources {
			wg.Add(1)

			go func(name string, source sources.Source) {
				defer wg.Done()

				for res := range source.Run(collector.keys, collector.filter) {
					URLs <- res
				}
			}(name, source)
		}

		wg.Wait()
	}()

	return
}
