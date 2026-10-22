package input_test

import (
	"strings"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/input"
)

const (
	asciiDomain   = "xn--mnchen-3ya.de"
	exampleDomain = "example.com"
	unicodeDomain = "münchen.de"
)

func TestNormalizeDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "unicode label", input: unicodeDomain, want: asciiDomain},
		{name: "uppercase unicode with trailing dot", input: "MÜNCHEN.DE.", want: asciiDomain},
		{name: "punycode tld", input: "xn--p1ai", want: "xn--p1ai"},
		{name: "punycode domain", input: asciiDomain, want: asciiDomain},
		{name: "cyrillic domain", input: "пример.рф", want: "xn--e1afmkfd.xn--p1ai"},
		{name: "uppercase ascii with trailing dot", input: "Example.COM.", want: exampleDomain},
		{name: "single label", input: "localhost", want: "localhost"},
		{name: "empty", input: "", wantErr: true},
		{name: "whitespace only", input: "   ", wantErr: true},
		{name: "empty label", input: "example..com", wantErr: true},
		{name: "leading dot", input: ".example.com", wantErr: true},
		{name: "double trailing dot", input: "example.com..", wantErr: true},
		{name: "leading hyphen", input: "-bad.com", wantErr: true},
		{name: "trailing hyphen", input: "bad-.com", wantErr: true},
		{name: "underscore", input: "exa_mple.com", wantErr: true},
		{name: "space", input: "exa mple.com", wantErr: true},
		{name: "invalid utf-8", input: string([]byte{'e', 'x', 'a', 0xff, 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm'}), wantErr: true},
		{name: "slash", input: "exa/mple.com", wantErr: true},
		{name: "overlong label", input: strings.Repeat("a", 64) + "." + exampleDomain, wantErr: true},
		{
			name: "overlong domain",
			input: strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." +
				strings.Repeat("c", 63) + "." + strings.Repeat("d", 63),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := input.NormalizeDomain(test.input)

			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeDomain(%q) = %q, want error", test.input, got)
				}

				return
			}

			if err != nil {
				t.Fatalf("NormalizeDomain(%q) error: %v", test.input, err)
			}

			if got != test.want {
				t.Fatalf("NormalizeDomain(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
