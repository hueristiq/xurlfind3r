package xurlfind3r

import (
	"testing"
)

const (
	testDomain            = "example.com"
	testApexURL           = "https://example.com/x"
	testHostPathURL       = "example.com/x"
	testUppercaseURL      = "HTTPS://EXAMPLE.COM/x"
	testSubdomainURL      = "https://sub.example.com/x"
	testUnderscoredSubURL = "https://foo_bar.example.com/x"
	testLookalikeURL      = "https://example.com.evil.com/x"
)

func TestValidateURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		target            string
		domain            string
		includeSubdomains bool
		wantURL           string
		wantValid         bool
	}{
		{
			name:      "apex https",
			target:    testApexURL,
			domain:    testDomain,
			wantURL:   testApexURL,
			wantValid: true,
		},
		{
			name:      "apex with port",
			target:    "http://example.com:8080/a?b=c",
			domain:    testDomain,
			wantURL:   "http://example.com:8080/a?b=c",
			wantValid: true,
		},
		{
			name:      "protocol relative",
			target:    "//example.com/x",
			domain:    testDomain,
			wantURL:   testApexURL,
			wantValid: true,
		},
		{
			name:      "scheme relative",
			target:    "://example.com/x",
			domain:    testDomain,
			wantURL:   testApexURL,
			wantValid: true,
		},
		{
			name:      "host only",
			target:    testHostPathURL,
			domain:    testDomain,
			wantURL:   testApexURL,
			wantValid: true,
		},
		{
			name:      "www without subdomains",
			target:    "https://www.example.com/x",
			domain:    testDomain,
			wantURL:   "https://www.example.com/x",
			wantValid: true,
		},
		{
			name:      "uppercase host",
			target:    testUppercaseURL,
			domain:    testDomain,
			wantURL:   testUppercaseURL,
			wantValid: true,
		},
		{
			name:              "subdomain excluded",
			target:            testSubdomainURL,
			domain:            testDomain,
			includeSubdomains: false,
			wantURL:           testSubdomainURL,
			wantValid:         false,
		},
		{
			name:              "subdomain included",
			target:            testSubdomainURL,
			domain:            testDomain,
			includeSubdomains: true,
			wantURL:           testSubdomainURL,
			wantValid:         true,
		},
		{
			name:              "underscored subdomain",
			target:            testUnderscoredSubURL,
			domain:            testDomain,
			includeSubdomains: true,
			wantURL:           testUnderscoredSubURL,
			wantValid:         true,
		},
		{
			name:      "lookalike host",
			target:    testLookalikeURL,
			domain:    testDomain,
			wantURL:   testLookalikeURL,
			wantValid: false,
		},
		{
			name:              "lookalike host with subdomains",
			target:            testLookalikeURL,
			domain:            testDomain,
			includeSubdomains: true,
			wantURL:           testLookalikeURL,
			wantValid:         false,
		},
		{
			name:      "query string smuggling",
			target:    "https://other.com/?next=https://example.com/a",
			domain:    testDomain,
			wantURL:   "https://other.com/?next=https://example.com/a",
			wantValid: false,
		},
		{
			name:      "fabricated userinfo host",
			target:    "http://www.evil.com/@example.com/x",
			domain:    testDomain,
			wantURL:   "http://www.evil.com/@example.com/x",
			wantValid: false,
		},
		{
			name:      "hyphen lookalike",
			target:    "https://evil-example.com/x",
			domain:    testDomain,
			wantURL:   "https://evil-example.com/x",
			wantValid: false,
		},
		{
			name:      "ip literal",
			target:    "https://127.0.0.1/x",
			domain:    testDomain,
			wantURL:   "https://127.0.0.1/x",
			wantValid: false,
		},
		{
			name:      "mailto is not a URL with host",
			target:    "mailto:user@example.com",
			domain:    testDomain,
			wantURL:   "mailto:user@example.com",
			wantValid: false,
		},
		{
			name:      "non-http scheme",
			target:    "ftp://example.com/x",
			domain:    testDomain,
			wantURL:   "ftp://example.com/x",
			wantValid: false,
		},
		{
			name:      "userinfo with in-scope host",
			target:    "https://user@example.com/x",
			domain:    testDomain,
			wantURL:   "https://user@example.com/x",
			wantValid: true,
		},
		{
			name:      "garbage input",
			target:    "://",
			domain:    testDomain,
			wantURL:   "https://",
			wantValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			URL, valid := validateURL(test.target, test.domain, test.includeSubdomains)

			if valid != test.wantValid {
				t.Errorf("valid = %t, want %t", valid, test.wantValid)
			}

			if URL != test.wantURL {
				t.Errorf("URL = %q, want %q", URL, test.wantURL)
			}
		})
	}
}

func TestCompileExtractor(t *testing.T) {
	t.Parallel()

	extractor, err := compileExtractor("example.com")
	if err != nil {
		t.Fatalf("compileExtractor: %v", err)
	}

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "apex is extracted",
			text: testApexURL,
			want: []string{testApexURL},
		},
		{
			name: "apex with port is extracted",
			text: "https://example.com:8080/x",
			want: []string{"https://example.com:8080/x"},
		},
		{
			name: "subdomain is extracted",
			text: "https://sub.example.com/x",
			want: []string{"https://sub.example.com/x"},
		},
		{
			name: "underscored subdomain is extracted",
			text: "https://foo_bar.example.com/x",
			want: []string{"https://foo_bar.example.com/x"},
		},
		{
			name: "uppercase host is extracted",
			text: "HTTPS://EXAMPLE.COM/x",
			want: []string{"HTTPS://EXAMPLE.COM/x"},
		},
		{
			name: "bare host is extracted",
			text: "example.com/x",
			want: []string{"example.com/x"},
		},
		{
			name: "surrounded by text",
			text: "see " + testApexURL + " for details",
			want: []string{testApexURL},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := extractor.FindAllString(test.text, -1)

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
