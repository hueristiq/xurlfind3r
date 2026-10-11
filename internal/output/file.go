package output

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

// Writer writes results to any io.Writer in TXT or JSONL format.
//
// Fields:
//   - format (format): the output format.
type Writer struct {
	format format
}

// SetFormatToJSONL switches the writer to JSONL output.
func (w *Writer) SetFormatToJSONL() {
	w.format = formatJSONL
}

// CreateFile opens the output file at path for appending, creating
// parent directories as needed. Paths without an extension get the
// format's default extension appended.
//
// Parameters:
//   - path (string): the output file path.
//
// Returns:
//   - file (*os.File): the opened file.
//   - err (error): non-nil on failure.
func (w *Writer) CreateFile(path string) (file *os.File, err error) {
	if path == "" {
		err = ErrNoFilePathSpecified

		return
	}

	if extension := filepath.Ext(path); extension == "" {
		switch w.format {
		case formatTXT:
			path += ".txt"
		case formatJSONL:
			path += ".jsonl"
		}
	}

	directory := filepath.Dir(path)

	if directory != "" {
		if _, err = os.Stat(directory); os.IsNotExist(err) {
			err = os.MkdirAll(directory, 0o750)
			if err != nil {
				return
			}
		}
	}

	//nolint:gosec // the path is user-provided via the --output/--output-directory flags; the file is created with 0600 permissions.
	file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}

	return
}

// Write writes one result to writer in the configured format.
//
// Parameters:
//   - writer (io.Writer): the destination.
//   - domain (string): the target domain, recorded in JSONL output.
//   - result (sources.Result): the result to write.
//
// Returns:
//   - err (error): non-nil on failure.
func (w *Writer) Write(writer io.Writer, domain string, result sources.Result) (err error) {
	switch w.format {
	case formatTXT:
		err = w.writeTXT(writer, result)
	case formatJSONL:
		err = w.writeJSON(writer, domain, result)
	}

	return
}

func (w *Writer) writeTXT(writer io.Writer, result sources.Result) (err error) {
	bw := bufio.NewWriter(writer)

	if _, err = fmt.Fprintln(bw, result.Value); err != nil {
		return
	}

	if err = bw.Flush(); err != nil {
		return
	}

	return
}

func (w *Writer) writeJSON(writer io.Writer, domain string, result sources.Result) (err error) {
	data := resultForJSONL{
		Domain: domain,
		URL:    result.Value,
		Source: result.Source,
	}

	var dataJSONBytes []byte

	dataJSONBytes, err = json.Marshal(data)
	if err != nil {
		return
	}

	dataJSONString := string(dataJSONBytes)

	bw := bufio.NewWriter(writer)

	if _, err = fmt.Fprintln(bw, dataJSONString); err != nil {
		return
	}

	if err = bw.Flush(); err != nil {
		return
	}

	return
}

type format string

type resultForJSONL struct {
	Domain string `json:"domain"`
	URL    string `json:"url"`
	Source string `json:"source"`
}

const (
	formatJSONL format = "JSON"
	formatTXT   format = "TXT"
)

// ErrNoFilePathSpecified is returned when an empty output path is
// given.
var ErrNoFilePathSpecified = errors.New("no file path specified")

// NewWriter returns a TXT-format writer.
//
// Returns:
//   - writer (*Writer): the new writer.
func NewWriter() (writer *Writer) {
	writer = &Writer{
		format: formatTXT,
	}

	return
}
