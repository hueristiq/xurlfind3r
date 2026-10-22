package output

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hueristiq/xurlfind3r/pkg/xurlfind3r/sources"
)

type Writer struct {
	format  format
	writers map[io.Writer]*bufio.Writer
}

func (w *Writer) SetFormatToJSONL() {
	w.format = formatJSONL
}

func (w *Writer) CreateFile(path string) (file *os.File, err error) {
	if path == "" {
		err = ErrNoFilePathSpecified

		return
	}

	extension := strings.ToLower(filepath.Ext(path))

	switch w.format {
	case formatTXT:
		if extension != ".txt" {
			path += ".txt"
		}
	case formatJSONL:
		if extension != ".json" && extension != ".jsonl" {
			path += ".json"
		}
	}

	directory := filepath.Dir(path)

	if directory != "" {
		if _, err = os.Stat(directory); os.IsNotExist(err) {
			if err = os.MkdirAll(directory, 0o750); err != nil {
				return
			}
		}
	}

	file, err = os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // path is the user-specified output file location.
	if err != nil {
		return
	}

	return
}

func (w *Writer) Write(writer io.Writer, domain string, result sources.Result) (err error) {
	bw := w.bufferedWriter(writer)

	switch w.format {
	case formatTXT:
		if _, err = bw.WriteString(result.Value + "\n"); err != nil {
			return
		}
	case formatJSONL:
		data := resultForJSONL{
			Domain: domain,
			URL:    result.Value,
			Source: result.Source,
		}

		encoder := json.NewEncoder(bw)
		encoder.SetEscapeHTML(false)

		if err = encoder.Encode(data); err != nil {
			return
		}
	}

	return
}

func (w *Writer) WriteWithSource(writer io.Writer, domain string, result sources.Result) (err error) {
	switch w.format {
	case formatTXT:
		bw := w.bufferedWriter(writer)

		if _, err = bw.WriteString("[" + result.Source + "] " + result.Value + "\n"); err != nil {
			return
		}
	case formatJSONL:
		if err = w.Write(writer, domain, result); err != nil {
			return
		}
	}

	return
}

func (w *Writer) Flush(writer io.Writer) (err error) {
	bw, ok := w.writers[writer]
	if !ok {
		return
	}

	if err = bw.Flush(); err != nil {
		return
	}

	return
}

func (w *Writer) Release(writer io.Writer) (err error) {
	bw, ok := w.writers[writer]
	if !ok {
		return
	}

	err = bw.Flush()

	delete(w.writers, writer)

	return
}

func (w *Writer) bufferedWriter(writer io.Writer) (bw *bufio.Writer) {
	bw, ok := w.writers[writer]
	if !ok {
		bw = bufio.NewWriterSize(writer, 256*1024)
		w.writers[writer] = bw
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
	formatJSONL format = "JSONL"
	formatTXT   format = "TXT"
)

var ErrNoFilePathSpecified = errors.New("no file path specified")

func NewWriter() (writer *Writer) {
	writer = &Writer{
		format:  formatTXT,
		writers: make(map[io.Writer]*bufio.Writer),
	}

	return
}
