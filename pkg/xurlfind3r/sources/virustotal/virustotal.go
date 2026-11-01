package virustotal

import (
	"context"
	"fmt"
	"net/http"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
	"github.com/hueristiq/xurlfind3r/internal/source/virustotal"
	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Source struct {
	keys sources.Keys
}

func (s *Source) Name() string {
	return sources.Virustotal
}

func (s *Source) NeedsKeys() bool {
	return true
}

func (s *Source) UseKeys(keys ...string) {
	s.keys = append(s.keys, keys...)
}

func (s *Source) Run(ctx context.Context, cfg *sources.Configuration, domain string) <-chan sources.Result {
	results := make(chan sources.Result)

	go func() {
		defer close(results)

		emit := func(result sources.Result) bool {
			select {
			case results <- result:
				return true
			case <-ctx.Done():
				return false
			}
		}

		key, err := s.keys.PickRandom()
		if err != nil {
			emit(sources.ErrorResult(s.Name(), fmt.Errorf("failed to select key: %w", err)))

			return
		}

		client, err := virustotal.New(hqgosource.DoerFunc(func(request *http.Request) (*http.Response, error) {
			return cfg.Client.Do(request)
		}), key)
		if err != nil {
			emit(sources.ErrorResult(s.Name(), err))

			return
		}

		for result := range client.DomainReport(ctx, domain) {
			if result.Err != nil {
				if !emit(sources.ErrorResult(s.Name(), result.Err)) {
					return
				}

				continue
			}

			URL, valid := cfg.Validator(result.Value)
			if !valid {
				continue
			}

			if !emit(sources.URLResult(s.Name(), URL)) {
				return
			}
		}
	}()

	return results
}

var _ sources.Source = (*Source)(nil)

func New() sources.Source {
	return &Source{keys: make(sources.Keys, 0)}
}
