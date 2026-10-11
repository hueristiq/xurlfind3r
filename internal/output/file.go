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

// bufferSize is the buffered output chunk size. A large buffer keeps
// the number of syscalls low on million-URL runs.
const bufferSize = 256 * 1024

// Writer writes results to any io.Writer in TXT or JSONL format.
//
// Each destination gets its own buffered writer, flushed by [Writer.Flush].
//
// Fields:
//   - format (format): the output format.
//   - buffers (map[io.Writer]*bufio.Writer): per-destination buffers.
type Writer struct {
	format  format
	buffers map[io.Writer]*bufio.Writer
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

// Write buffers one result for writer in the configured format.
//
// Parameters:
//   - writer (io.Writer): the destination.
//   - domain (string): the target domain, recorded in JSONL output.
//   - result (sources.Result): the result to write.
//
// Returns:
//   - err (error): non-nil on failure.
func (w *Writer) Write(writer io.Writer, domain string, result sources.Result) (err error) {
	bw := w.buffer(writer)

	switch w.format {
	case formatTXT:
		err = writeTXT(bw, result)
	case formatJSONL:
		err = writeJSON(bw, domain, result)
	}

	return
}

// Flush flushes every destination's pending output.
//
// Returns:
//   - err (error): the joined flush failures, if any.
func (w *Writer) Flush() (err error) {
	for _, bw := range w.buffers {
		if ferr := bw.Flush(); ferr != nil {
			err = errors.Join(err, ferr)
		}
	}

	return
}

// buffer returns the buffered writer for destination, creating one on
// first use.
func (w *Writer) buffer(writer io.Writer) (bw *bufio.Writer) {
	if w.buffers == nil {
		w.buffers = make(map[io.Writer]*bufio.Writer)
	}

	bw, ok := w.buffers[writer]
	if !ok {
		bw = bufio.NewWriterSize(writer, bufferSize)

		w.buffers[writer] = bw
	}

	return bw
}

func writeTXT(writer io.Writer, result sources.Result) (err error) {
	if _, err = fmt.Fprintln(writer, result.Value); err != nil {
		return
	}

	return
}

func writeJSON(writer io.Writer, domain string, result sources.Result) (err error) {
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

	if _, err = fmt.Fprintln(writer, string(dataJSONBytes)); err != nil {
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
		format:  formatTXT,
		buffers: make(map[io.Writer]*bufio.Writer),
	}

	return
}
