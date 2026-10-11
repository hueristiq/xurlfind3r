package sources

import (
	"testing"

	hqgourlextractor "github.com/hueristiq/hq-lib-url-go/extractor"
)

const (
	testApexURL       = "https://example.com/x"
	testApexHost      = "https://example.com"
	testPortURL       = "https://example.com:8080/x"
	testSubdomainURL  = "https://sub.example.com/x"
	testBareHost      = "example.com/x"
	testHyphenURL     = "https://example.com-x"
	testEmail         = "user@example.com.evil.com"
	testLookalike     = "https://example.com.evil.com/x"
	testLookalikePort = "https://example.com.evil.com:443/x"
	testLookalikeSub  = "https://sub.example.com.evil.com/x"
)

func TestExtractURLs(t *testing.T) {
	t.Parallel()

	extractor, err := hqgourlextractor.New(
		hqgourlextractor.WithHostPattern(`(?:(?:\w+[.])*example\.com` + hqgourlextractor.PortOptionalPattern + `)`),
	).CompileRegex()
	if err != nil {
		t.Fatalf("failed to compile extractor: %v", err)
	}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "lookalike host is not extracted",
			text: testLookalike,
			want: nil,
		},
		{
			name: "lookalike host with port is not extracted",
			text: testLookalikePort,
			want: nil,
		},
		{
			name: "lookalike with subdomain label is not extracted",
			text: testLookalikeSub,
			want: nil,
		},
		{
			name: "host prefix of a hyphenated label is not extracted",
			text: testHyphenURL,
			want: nil,
		},
		{
			name: "lookalike email is not extracted",
			text: testEmail,
			want: nil,
		},
		{
			name: "apex is extracted",
			text: testApexURL,
			want: []string{testApexURL},
		},
		{
			name: "apex with port is extracted",
			text: testPortURL,
			want: []string{testPortURL},
		},
		{
			name: "subdomain is extracted",
			text: testSubdomainURL,
			want: []string{testSubdomainURL},
		},
		{
			name: "bare host is extracted",
			text: testBareHost,
			want: []string{testBareHost},
		},
		{
			name: "host at end of text is extracted",
			text: testApexHost,
			want: []string{testApexHost},
		},
		{
			name: "query-only URL keeps its host",
			text: testApexHost + "?x=1",
			want: []string{testApexHost},
		},
		{
			name: "host followed by punctuation is extracted",
			text: "see " + testApexURL + ", then leave",
			want: []string{testApexURL},
		},
		{
			name: "multiple matches",
			text: "https://example.com/a and https://sub.example.com/b",
			want: []string{"https://example.com/a", "https://sub.example.com/b"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := ExtractURLs(extractor, test.text)

			if len(got) != len(test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}

			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}
