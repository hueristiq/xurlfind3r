package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

const (
	testURL1 = "https://example.com/a"
	testURL2 = "https://example.com/b"
)

func TestWriterTXT(t *testing.T) {
	t.Parallel()

	writer := NewWriter()

	var buffer bytes.Buffer

	for _, value := range []string{testURL1, testURL2} {
		err := writer.Write(&buffer, "example.com", sources.Result{
			Type:  sources.ResultURL,
			Value: value,
		})
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	want := testURL1 + "\n" + testURL2 + "\n"

	if buffer.String() != want {
		t.Errorf("output = %q, want %q", buffer.String(), want)
	}
}

func TestWriterJSONL(t *testing.T) {
	t.Parallel()

	writer := NewWriter()

	writer.SetFormatToJSONL()

	var buffer bytes.Buffer

	err := writer.Write(&buffer, "example.com", sources.Result{
		Type:   sources.ResultURL,
		Source: "wayback",
		Value:  testURL1,
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var record resultForJSONL

	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &record); err != nil {
		t.Fatalf("failed to unmarshal JSONL output: %v", err)
	}

	if record.Domain != "example.com" || record.URL != testURL1 || record.Source != "wayback" {
		t.Errorf("record = %+v, want example.com/%s/wayback", record, testURL1)
	}
}

func TestWriterBuffersUntilFlush(t *testing.T) {
	t.Parallel()

	writer := NewWriter()

	var buffer bytes.Buffer

	if err := writer.Write(&buffer, "example.com", sources.Result{Type: sources.ResultURL, Value: testURL1}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if buffer.Len() != 0 {
		t.Errorf("buffer has %d bytes before Flush, want 0", buffer.Len())
	}

	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if buffer.Len() == 0 {
		t.Errorf("buffer is empty after Flush")
	}
}

func TestCreateFileExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format format
		path   string
		want   string
	}{
		{
			name:   "txt default extension",
			format: formatTXT,
			path:   "out",
			want:   "out.txt",
		},
		{
			name:   "txt keeps user extension",
			format: formatTXT,
			path:   "out.log",
			want:   "out.log",
		},
		{
			name:   "jsonl default extension",
			format: formatJSONL,
			path:   "out",
			want:   "out.jsonl",
		},
		{
			name:   "jsonl keeps user extension",
			format: formatJSONL,
			path:   "out.custom",
			want:   "out.custom",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()

			writer := NewWriter()

			writer.format = test.format

			file, err := writer.CreateFile(filepath.Join(directory, test.path))
			if err != nil {
				t.Fatalf("CreateFile: %v", err)
			}

			if err := file.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			if _, err := os.Stat(filepath.Join(directory, test.want)); err != nil {
				t.Errorf("expected file %s: %v", test.want, err)
			}
		})
	}
}

func TestCreateFileEmptyPath(t *testing.T) {
	t.Parallel()

	writer := NewWriter()

	_, err := writer.CreateFile("")
	if err == nil {
		t.Fatalf("CreateFile: got nil error, want %v", ErrNoFilePathSpecified)
	}

	if !strings.Contains(err.Error(), "no file path") {
		t.Errorf("error = %v, want it to mention the missing path", err)
	}
}
