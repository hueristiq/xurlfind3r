//nolint:testpackage // white-box tests for unexported helpers.
package xurlfind3r

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	exampleDomain  = "example.com"
	fakeSourceName = "fake"
)

type fakeSource struct {
	name string
	run  func(ctx context.Context, cfg *sources.Configuration) <-chan sources.Result
}

func (f *fakeSource) Name() (name string) {
	name = f.name

	return
}

func (f *fakeSource) NeedsKeys() (needs bool) {
	needs = false

	return
}

func (f *fakeSource) UseKeys(_ ...string) {
}

func (f *fakeSource) Run(ctx context.Context, cfg *sources.Configuration, _ string) <-chan sources.Result {
	return f.run(ctx, cfg)
}

func newStaticSource(name string, values ...string) (source *fakeSource) {
	source = &fakeSource{
		name: name,
		run: func(_ context.Context, cfg *sources.Configuration) <-chan sources.Result {
			results := make(chan sources.Result, len(values))

			for _, value := range values {
				if URL, valid := cfg.Validator(value); valid {
					results <- sources.URLResult(name, URL)
				}
			}

			close(results)

			return results
		},
	}

	return
}

func TestSourceRegistry(t *testing.T) {
	t.Parallel()

	for _, name := range sources.List {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, ok := NewSource(name)
			if !ok {
				t.Fatalf("NewSource(%q) not registered", name)
			}

			if got := source.Name(); got != name {
				t.Fatalf("source.Name() = %q, want %q", got, name)
			}

			source.UseKeys("test-key")
		})
	}
}

func TestHostPattern(t *testing.T) {
	t.Parallel()

	extractor := regexp.MustCompile("^(?:" + hostPattern(exampleDomain) + ")$")

	tests := []struct {
		name string
		host string
		want bool
	}{
		{name: "plain", host: exampleDomain, want: true},
		{name: "subdomain", host: "sub." + exampleDomain, want: true},
		{name: "hyphenated label", host: "foo-bar." + exampleDomain, want: true},
		{name: "punycode label", host: "xn--mnchen-3ya." + exampleDomain, want: true},
		{name: "single character label", host: "a." + exampleDomain, want: true},
		{name: "port", host: exampleDomain + ":8443", want: true},
		{name: "leading hyphen", host: "-bad." + exampleDomain, want: false},
		{name: "trailing hyphen", host: "bad-." + exampleDomain, want: false},
		{name: "longer hostname", host: exampleDomain + ".evil.com", want: false},
		{name: "glued prefix", host: "barfoo" + exampleDomain, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := extractor.MatchString(test.host); got != test.want {
				t.Fatalf("hostPattern match %q = %v, want %v", test.host, got, test.want)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
		value string
	}{
		{name: "https", input: "https://" + exampleDomain, want: true, value: "https://" + exampleDomain},
		{
			name:  "scheme-less host and port",
			input: exampleDomain + ":8443/login",
			want:  true,
			value: "https://" + exampleDomain + ":8443/login",
		},
		{
			name:  "double slash in query",
			input: exampleDomain + "/path?next=https://evil.com",
			want:  true,
			value: "https://" + exampleDomain + "/path?next=https://evil.com",
		},
		{name: "protocol relative", input: "//" + exampleDomain, want: true, value: "https://" + exampleDomain},
		{name: "missing scheme marker", input: "://" + exampleDomain, want: true, value: "https://" + exampleDomain},
		{
			name:  "uppercase host and explicit port",
			input: "https://EXAMPLE.COM:443/Path",
			want:  true,
			value: "https://example.com:443/Path",
		},
		{name: "www", input: "https://www." + exampleDomain, want: true, value: "https://www." + exampleDomain},
		{name: "subdomain outside scope", input: "https://sub." + exampleDomain, want: false},
		{name: "mailto", input: "mailto:user@" + exampleDomain, want: false},
		{name: "ftp", input: "ftp://" + exampleDomain, want: false},
		{name: "userinfo", input: "https://user:pass@" + exampleDomain, want: false},
		{name: "suffix trap", input: "https://" + exampleDomain + ".evil.com", want: false},
		{name: "trailing dot", input: "https://" + exampleDomain + ".", want: false},
		{name: "invalid port", input: "https://" + exampleDomain + ":bad", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, valid := validateURL(test.input, exampleDomain, false)

			if valid != test.want {
				t.Fatalf("validateURL(%q) = (%q, %v), want valid=%v", test.input, got, valid, test.want)
			}

			if valid && got != test.value {
				t.Fatalf("validateURL(%q) = %q, want %q", test.input, got, test.value)
			}
		})
	}
}

func TestValidateURLIncludeSubdomains(t *testing.T) {
	t.Parallel()

	input := "https://sub." + exampleDomain + "/x"

	got, valid := validateURL(input, exampleDomain, true)
	if !valid || got != input {
		t.Fatalf("validateURL(%q, includeSubdomains) = (%q, %v)", input, got, valid)
	}
}

func TestCanonicalURLKey(t *testing.T) {
	t.Parallel()

	assertSameKey := func(t *testing.T, values ...string) {
		t.Helper()

		key := canonicalURLKey(values[0])

		for _, value := range values[1:] {
			if got := canonicalURLKey(value); got != key {
				t.Fatalf("canonicalURLKey(%q) = %q, want %q", value, got, key)
			}
		}
	}

	assertDifferentKey := func(t *testing.T, left, right string) {
		t.Helper()

		if canonicalURLKey(left) == canonicalURLKey(right) {
			t.Fatalf("canonicalURLKey(%q) == canonicalURLKey(%q)", left, right)
		}
	}

	t.Run("case and default ports", func(t *testing.T) {
		t.Parallel()

		assertSameKey(t,
			"https://Example.COM",
			"https://example.com/",
			"https://example.com:443/",
			"https://example.com/#frag",
		)
	})

	t.Run("query order", func(t *testing.T) {
		t.Parallel()

		assertSameKey(t,
			"https://example.com/?b=2&a=1",
			"https://example.com/?a=1&b=2",
		)
	})

	t.Run("ipv6 default port", func(t *testing.T) {
		t.Parallel()

		assertSameKey(t,
			"http://[::1]:80/x",
			"http://[::1]/x",
		)
	})

	t.Run("scheme is preserved", func(t *testing.T) {
		t.Parallel()

		assertDifferentKey(t, "http://"+exampleDomain+"/x", "https://"+exampleDomain+"/x")
	})

	t.Run("scheme-specific default ports", func(t *testing.T) {
		t.Parallel()

		assertDifferentKey(t, "https://"+exampleDomain+":80/x", "https://"+exampleDomain+"/x")
		assertDifferentKey(t, "http://"+exampleDomain+":443/x", "http://"+exampleDomain+"/x")
	})

	t.Run("path and query are significant", func(t *testing.T) {
		t.Parallel()

		assertDifferentKey(t, "https://"+exampleDomain+"/x", "https://"+exampleDomain+"/x/")
		assertDifferentKey(t, "https://"+exampleDomain+"/?a=1", "https://"+exampleDomain+"/")
	})
}

func TestNewErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *Configuration
		wantErr string
	}{
		{name: "nil configuration", cfg: nil, wantErr: "configuration is nil"},
		{
			name:    "unknown source",
			cfg:     &Configuration{SourcesToUse: []string{"unknown"}},
			wantErr: "unknown source",
		},
		{
			name:    "unknown exclusion",
			cfg:     &Configuration{SourcesToExclude: []string{"unknown"}},
			wantErr: "in sources to exclude",
		},
		{
			name: "all sources excluded",
			cfg: &Configuration{
				SourcesToUse:     []string{sources.Wayback},
				SourcesToExclude: []string{sources.Wayback},
			},
			wantErr: "no sources left",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			finder, err := New(test.cfg)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("New() = (%v, %v), want error containing %q", finder, err, test.wantErr)
			}
		})
	}
}

func TestSourceCount(t *testing.T) {
	t.Parallel()

	finder, err := New(&Configuration{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if got, want := finder.SourceCount(), len(sources.List); got != want {
		t.Fatalf("SourceCount() = %d, want %d", got, want)
	}

	finder, err = New(&Configuration{
		SourcesToUse:     []string{sources.Wayback, sources.OpenThreatExchange},
		SourcesToExclude: []string{sources.OpenThreatExchange},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if got := finder.SourceCount(); got != 1 {
		t.Fatalf("SourceCount() = %d, want 1", got)
	}
}

func TestFindScopeAndDedup(t *testing.T) {
	t.Parallel()

	finder := &Finder{
		sources: map[string]sources.Source{
			fakeSourceName: newStaticSource(fakeSourceName,
				"https://"+exampleDomain+"/a",
				"https://"+exampleDomain+"/a#frag",
				"https://"+exampleDomain+":443/a",
				"https://EXAMPLE.COM/a",
				"https://"+exampleDomain+"/b",
				"https://sub."+exampleDomain+"/c",
				"https://"+exampleDomain+".evil.com/d",
				"mailto:user@"+exampleDomain,
			),
		},
	}

	var got []string

	for result := range finder.Find(t.Context(), exampleDomain) {
		if result.Type == sources.ResultURL {
			got = append(got, result.Value)
		}
	}

	want := []string{
		"https://" + exampleDomain + "/a",
		"https://" + exampleDomain + "/b",
	}

	if len(got) != len(want) {
		t.Fatalf("Find() = %v, want %v", got, want)
	}

	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("Find() = %v, want %v", got, want)
		}
	}
}

func TestFindCancellationDrainsSources(t *testing.T) {
	t.Parallel()

	source := &fakeSource{
		name: fakeSourceName,
		run: func(_ context.Context, _ *sources.Configuration) <-chan sources.Result {
			results := make(chan sources.Result)

			go func() {
				defer close(results)

				for index := range 1000 {
					results <- sources.URLResult(fakeSourceName, fmt.Sprintf("https://%s/%d", exampleDomain, index))
				}
			}()

			return results
		},
	}

	finder := &Finder{
		sources: map[string]sources.Source{fakeSourceName: source},
	}

	ctx, cancel := context.WithCancel(t.Context())
	results := finder.Find(ctx, exampleDomain)

	if _, ok := <-results; !ok {
		t.Fatal("Find() closed before emitting a result")
	}

	cancel()

	done := make(chan struct{})

	go func() {
		for range results {
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Find() did not drain source channels after cancellation")
	}
}
