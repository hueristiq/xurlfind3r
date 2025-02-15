package formatter

import (
	"bytes"
)

type Console struct{}

func (c *Console) Format(log *Log) (data []byte, err error) {
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

var _ Formatter = (*Console)(nil)

func NewConsoleFormatter() (formatter *Console) {
	formatter = &Console{}

	return
}
