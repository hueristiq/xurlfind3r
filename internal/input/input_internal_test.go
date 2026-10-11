package input

import (
	"testing"
)

const (
	testDomain     = "example.com"
	testSubdomain  = "foo_bar.example.com"
	testUppercase  = "EXAMPLE.COM"
	testTraversal  = "../../tmp/x"
	testEmpty      = ""
	testBlank      = "   "
	testWithSpace  = "exa mple.com"
	testWithPath   = "example.com/path"
	testWithQuery  = "example.com?x=1"
	testWithScheme = "https://example.com"
)

func TestValidateDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{
			name:    "plain domain",
			domain:  testDomain,
			want:    testDomain,
			wantErr: false,
		},
		{
			name:    "trims whitespace",
			domain:  " " + testDomain + "\t",
			want:    testDomain,
			wantErr: false,
		},
		{
			name:    "underscored subdomain",
			domain:  testSubdomain,
			want:    testSubdomain,
			wantErr: false,
		},
		{
			name:    "uppercase",
			domain:  testUppercase,
			want:    testUppercase,
			wantErr: false,
		},
		{
			name:    "path traversal",
			domain:  testTraversal,
			wantErr: true,
		},
		{
			name:    "empty",
			domain:  testEmpty,
			wantErr: true,
		},
		{
			name:    "blank",
			domain:  testBlank,
			wantErr: true,
		},
		{
			name:    "space inside",
			domain:  testWithSpace,
			wantErr: true,
		},
		{
			name:    "path separator",
			domain:  testWithPath,
			wantErr: true,
		},
		{
			name:    "query characters",
			domain:  testWithQuery,
			wantErr: true,
		},
		{
			name:    "scheme prefix",
			domain:  testWithScheme,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ValidateDomain(test.domain)

			if test.wantErr {
				if err == nil {
					t.Fatalf("got %q and nil error, want error", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("ValidateDomain(%q): %v", test.domain, err)
			}

			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
