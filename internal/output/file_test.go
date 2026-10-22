//nolint:testpackage // white-box tests for the buffered writer map.
package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	exampleDomain  = "example.com"
	jsonExtension  = ".json"
	jsonlExtension = ".jsonl"
	txtExtension   = ".txt"
)

func TestCreateFileExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		jsonl bool
		path  string
		want  string
	}{
		{name: "txt lowercase", path: "out" + txtExtension, want: "out" + txtExtension},
		{name: "txt uppercase", path: "OUT.TXT", want: "OUT.TXT"},
		{name: "txt missing extension", path: "out", want: "out" + txtExtension},
		{name: "txt unknown extension", path: "out" + jsonExtension, want: "out" + jsonExtension + txtExtension},
		{name: "jsonl lowercase", jsonl: true, path: "out" + jsonExtension, want: "out" + jsonExtension},
		{name: "jsonl alternate extension", jsonl: true, path: "out" + jsonlExtension, want: "out" + jsonlExtension},
		{name: "jsonl uppercase", jsonl: true, path: "OUT.JSONL", want: "OUT.JSONL"},
		{name: "jsonl missing extension", jsonl: true, path: "out", want: "out" + jsonExtension},
		{name: "jsonl unknown extension", jsonl: true, path: "out" + txtExtension, want: "out" + txtExtension + jsonExtension},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writer := NewWriter()

			if test.jsonl {
				writer.SetFormatToJSONL()
			}

			file, err := writer.CreateFile(filepath.Join(dir, test.path))
			if err != nil {
				t.Fatalf("CreateFile() error: %v", err)
			}

			if err := file.Close(); err != nil {
				t.Fatalf("Close() error: %v", err)
			}

			if got := filepath.Base(file.Name()); got != test.want {
				t.Fatalf("created %q, want %q", got, test.want)
			}
		})
	}
}

func TestCreateFileEmptyPath(t *testing.T) {
	t.Parallel()

	if _, err := NewWriter().CreateFile(""); !errors.Is(err, ErrNoFilePathSpecified) {
		t.Fatalf("CreateFile(\"\") error = %v, want %v", err, ErrNoFilePathSpecified)
	}
}

func TestCreateFileCreatesDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "out")

	file, err := NewWriter().CreateFile(path)
	if err != nil {
		t.Fatalf("CreateFile() error: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "nested", "out"+txtExtension)); err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
}

func TestWriteTXT(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	writer := NewWriter()
	result := sources.URLResult(sources.Wayback, "https://"+exampleDomain+"/a")

	if err := writer.Write(&buffer, exampleDomain, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if buffer.Len() != 0 {
		t.Fatalf("Write() flushed early: %q", buffer.String())
	}

	if err := writer.Flush(&buffer); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}

	if got, want := buffer.String(), "https://"+exampleDomain+"/a\n"; got != want {
		t.Fatalf("TXT output = %q, want %q", got, want)
	}

	buffer.Reset()

	if err := writer.WriteWithSource(&buffer, exampleDomain, result); err != nil {
		t.Fatalf("WriteWithSource() error: %v", err)
	}

	if err := writer.Flush(&buffer); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}

	if got, want := buffer.String(), "[wayback] https://"+exampleDomain+"/a\n"; got != want {
		t.Fatalf("TXT output with source = %q, want %q", got, want)
	}
}

func TestWriteJSONL(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	writer := NewWriter()
	writer.SetFormatToJSONL()

	result := sources.URLResult(sources.Wayback, "https://"+exampleDomain+"/?a=1&b=2")

	if err := writer.Write(&buffer, exampleDomain, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if err := writer.Flush(&buffer); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}

	if strings.Contains(buffer.String(), `\u0026`) {
		t.Fatalf("JSONL escaped HTML: %q", buffer.String())
	}

	var decoded resultForJSONL

	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.Domain != exampleDomain {
		t.Fatalf("Domain = %q, want %q", decoded.Domain, exampleDomain)
	}

	if decoded.URL != result.Value {
		t.Fatalf("URL = %q, want %q", decoded.URL, result.Value)
	}

	if decoded.Source != sources.Wayback {
		t.Fatalf("Source = %q, want %q", decoded.Source, sources.Wayback)
	}
}

func TestReleaseFlushesAndEvicts(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	writer := NewWriter()
	result := sources.URLResult(sources.Wayback, "https://"+exampleDomain+"/a")

	if err := writer.Write(&buffer, exampleDomain, result); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if len(writer.writers) != 1 {
		t.Fatalf("writers = %d, want 1", len(writer.writers))
	}

	if err := writer.Release(&buffer); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	if got, want := buffer.String(), "https://"+exampleDomain+"/a\n"; got != want {
		t.Fatalf("released output = %q, want %q", got, want)
	}

	if len(writer.writers) != 0 {
		t.Fatalf("writers = %d after Release, want 0", len(writer.writers))
	}

	if err := writer.Release(&buffer); err != nil {
		t.Fatalf("second Release() error: %v", err)
	}
}
