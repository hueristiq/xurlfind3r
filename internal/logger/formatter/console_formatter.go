package formatter

import (
	"bytes"

	"github.com/hueristiq/xurlfind3r/internal/logger/levels"
	"github.com/logrusorgru/aurora/v4"
)

type Console struct {
	au *aurora.Aurora
}

func (c *Console) Format(log *Log) (data []byte, err error) {
	c.colorize(log)

	buffer := &bytes.Buffer{}

	buffer.Grow(len(log.Message))

	if label, ok := log.Metadata["label"]; ok && label != "" {
		buffer.WriteByte('[')
		buffer.WriteString(label)
		buffer.WriteByte(']')
		buffer.WriteByte(' ')
	}

	buffer.WriteString(log.Message)

	data = buffer.Bytes()

	return
}

func (c *Console) colorize(log *Log) {
	label := log.Metadata["label"]

	if label == "" {
		return
	}

	//nolint: exhaustive
	switch log.Level {
	case levels.LevelFatal:
		log.Metadata["label"] = c.au.BrightRed(label).Bold().String()
	case levels.LevelError:
		log.Metadata["label"] = c.au.BrightRed(label).Bold().String()
	case levels.LevelInfo:
		log.Metadata["label"] = c.au.BrightBlue(label).Bold().String()
	case levels.LevelWarn:
		log.Metadata["label"] = c.au.BrightYellow(label).Bold().String()
	case levels.LevelDebug:
		log.Metadata["label"] = c.au.BrightMagenta(label).Bold().String()
	}
}

type ConsoleFormatterConfiguration struct {
	Colorize bool
}

var _ Formatter = (*Console)(nil)

func NewConsoleFormatter(cfg *ConsoleFormatterConfiguration) (formatter *Console) {
	formatter = &Console{
		au: aurora.New(aurora.WithColors(cfg.Colorize)),
	}

	return
}
