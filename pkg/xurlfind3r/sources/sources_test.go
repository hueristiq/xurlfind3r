package sources_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	exampleDomain = "example.com"
	exampleURL    = "https://example.com"
	okBody        = "ok"
)

func TestCheckResponse(t *testing.T) {
	t.Parallel()

	longBodyPrefix := "unexpected status 500 Internal Server Error: "

	tests := []struct {
		name           string
		statusCode     int
		body           string
		nilBody        bool
		nilResponse    bool
		secrets        []string
		wantErr        bool
		wantContains   string
		wantNotContain string
		maxLen         int
	}{
		{name: "nil response", nilResponse: true, wantErr: true, wantContains: "nil response"},
		{name: "status ok", statusCode: http.StatusOK, body: okBody},
		{name: "created", statusCode: http.StatusCreated, body: okBody},
		{
			name:         "not found",
			statusCode:   http.StatusNotFound,
			body:         "  not found  ",
			wantErr:      true,
			wantContains: "unexpected status 404 Not Found: not found",
		},
		{
			name:         "redirect",
			statusCode:   http.StatusMovedPermanently,
			body:         "moved",
			wantErr:      true,
			wantContains: "unexpected status 301 Moved Permanently",
		},
		{
			name:           "redacts secrets",
			statusCode:     http.StatusForbidden,
			body:           "token secret denied",
			secrets:        []string{"secret"},
			wantErr:        true,
			wantContains:   "[REDACTED]",
			wantNotContain: "secret",
		},
		{
			name:         "bounds body",
			statusCode:   http.StatusInternalServerError,
			body:         strings.Repeat("x", 5000),
			wantErr:      true,
			wantContains: longBodyPrefix,
			maxLen:       len(longBodyPrefix) + 4096,
		},
		{
			name:         "nil body",
			statusCode:   http.StatusBadGateway,
			nilBody:      true,
			wantErr:      true,
			wantContains: "unexpected status 502 Bad Gateway",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var response *http.Response

			if !test.nilResponse {
				response = &http.Response{
					StatusCode: test.statusCode,
					Status:     fmt.Sprintf("%d %s", test.statusCode, http.StatusText(test.statusCode)),
					Body:       io.NopCloser(strings.NewReader(test.body)),
				}

				if test.nilBody {
					response.Body = nil
				} else {
					defer func() {
						_ = response.Body.Close()
					}()
				}
			}

			err := sources.CheckResponse(response, test.secrets...)

			if !test.wantErr {
				if err != nil {
					t.Fatalf("CheckResponse() error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatal("CheckResponse() = nil error, want error")
			}

			message := err.Error()

			if test.wantContains != "" && !strings.Contains(message, test.wantContains) {
				t.Fatalf("CheckResponse() = %q, want substring %q", message, test.wantContains)
			}

			if test.wantNotContain != "" && strings.Contains(message, test.wantNotContain) {
				t.Fatalf("CheckResponse() = %q, must not contain %q", message, test.wantNotContain)
			}

			if test.maxLen > 0 && len(message) > test.maxLen {
				t.Fatalf("error message length = %d, want <= %d", len(message), test.maxLen)
			}
		})
	}
}

func TestResultConstructors(t *testing.T) {
	t.Parallel()

	urlResult := sources.URLResult(sources.Wayback, exampleURL)

	if urlResult.Type != sources.ResultURL || urlResult.Source != sources.Wayback || urlResult.Value != exampleURL {
		t.Fatalf("URLResult() = %+v", urlResult)
	}

	warning := sources.WarningResult(sources.Wayback, "capped")

	if warning.Type != sources.ResultWarning || warning.Source != sources.Wayback || warning.Value != "capped" {
		t.Fatalf("WarningResult() = %+v", warning)
	}

	boom := errors.New("boom")
	errorResult := sources.ErrorResult(sources.Wayback, boom)

	if errorResult.Type != sources.ResultError || errorResult.Source != sources.Wayback || !errors.Is(errorResult.Error, boom) {
		t.Fatalf("ErrorResult() = %+v", errorResult)
	}
}

func TestRedactError(t *testing.T) {
	t.Parallel()

	original := errors.New("token abc and a+b")

	if got := sources.RedactError(original); !errors.Is(got, original) {
		t.Fatalf("RedactError() without secrets = %v, want original error", got)
	}

	redacted := sources.RedactError(original, "abc", "a b")

	message := redacted.Error()
	if strings.Contains(message, "abc") || strings.Contains(message, "a+b") {
		t.Fatalf("RedactError() = %q, want both forms redacted", message)
	}

	if !strings.Contains(message, "[REDACTED]") {
		t.Fatalf("RedactError() = %q, want redaction marker", message)
	}

	if !errors.Is(redacted, original) {
		t.Fatal("RedactError() does not unwrap to the original error")
	}
}

func TestSendURLs(t *testing.T) {
	t.Parallel()

	results := make(chan sources.Result, 3)

	cfg := &sources.Configuration{
		Validator: func(target string) (string, bool) {
			if strings.Contains(target, "evil") {
				return target, false
			}

			return target, true
		},
	}

	sources.SendURLs(results, cfg, sources.Wayback, []string{
		"https://" + exampleDomain + "/a",
		"https://" + exampleDomain + "/evil",
		"https://" + exampleDomain + "/b",
	})

	close(results)

	var collected []sources.Result

	for result := range results {
		collected = append(collected, result)
	}

	if len(collected) != 2 {
		t.Fatalf("SendURLs() emitted %d results, want 2", len(collected))
	}

	if collected[0].Value != "https://"+exampleDomain+"/a" || collected[1].Value != "https://"+exampleDomain+"/b" {
		t.Fatalf("SendURLs() values = %v", collected)
	}

	if collected[0].Source != sources.Wayback {
		t.Fatalf("SendURLs() source = %q, want %q", collected[0].Source, sources.Wayback)
	}
}

func TestExtractURLsBoundary(t *testing.T) {
	t.Parallel()

	extractor := regexp.MustCompile(`https://example\.com`)

	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "sentence period", text: "see " + exampleURL + ". next", want: []string{exampleURL}},
		{name: "end of input", text: exampleURL, want: []string{exampleURL}},
		{name: "closing punctuation", text: "(" + exampleURL + ").", want: []string{exampleURL}},
		{name: "longer hostname", text: exampleURL + ".evil.com", want: nil},
		{name: "leading host label", text: "x" + exampleURL, want: nil},
		{name: "trailing hyphen", text: exampleURL + "-evil", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := sources.ExtractURLs(extractor, test.text)

			if !slices.Equal(got, test.want) {
				t.Fatalf("ExtractURLs(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestExtractURLsEncodedSlashPrefix(t *testing.T) {
	t.Parallel()

	extractor := regexp.MustCompile(`(?i)(?:[a-z0-9_](?:[a-z0-9_-]*[a-z0-9_])?\.)*example\.com`)

	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "encoded slash prefix", text: "%2Fwww.example.com", want: []string{"www.example.com"}},
		{name: "encoded slash prefix lowercase", text: "%2fwww.example.com", want: []string{"www.example.com"}},
		{name: "encoded slash empty label", text: "%2f.example.com", want: nil},
		{name: "legitimate two f label", text: "2f.example.com", want: []string{"2f.example.com"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := sources.ExtractURLs(extractor, test.text)

			if !slices.Equal(got, test.want) {
				t.Fatalf("ExtractURLs(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestExtractURLsTrimsCodeGlue(t *testing.T) {
	t.Parallel()

	const reportPDFURL = "https://example.com/report.pdf"

	extractor := regexp.MustCompile(`https?://\S*`)

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "quote and list tail",
			text: `https://example.com/investor-relations/',a['href']`,
			want: []string{"https://example.com/investor-relations/"},
		},
		{
			name: "dict tail",
			text: `https://example.com/investor-relations/','download_url':url,'http_status':'linked`,
			want: []string{"https://example.com/investor-relations/"},
		},
		{
			name: "status column",
			text: reportPDFURL + ",linked",
			want: []string{reportPDFURL},
		},
		{
			name: "numeric status columns",
			text: reportPDFURL + ",200,application/pdf",
			want: []string{reportPDFURL},
		},
		{
			name: "method column",
			text: reportPDFURL + ",GET",
			want: []string{reportPDFURL},
		},
		{
			name: "glued second url",
			text: "https://example.com/a,https://example.com/b",
			want: []string{"https://example.com/a", "https://example.com/b"},
		},
		{
			name: "legitimate comma path",
			text: "https://example.com/tags/foo,bar",
			want: []string{"https://example.com/tags/foo,bar"},
		},
		{
			name: "apostrophe path",
			text: "https://example.com/company's-report",
			want: []string{"https://example.com/company's-report"},
		},
		{
			name: "trailing slash column",
			text: "https://example.com/dir/,linked",
			want: []string{"https://example.com/dir/"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := sources.ExtractURLs(extractor, test.text)

			if !slices.Equal(got, test.want) {
				t.Fatalf("ExtractURLs(%q) = %v, want %v", test.text, got, test.want)
			}
		})
	}
}

func TestMaxPages(t *testing.T) {
	t.Parallel()

	if sources.MaxPages != 100 {
		t.Fatalf("MaxPages = %d, want 100", sources.MaxPages)
	}
}
