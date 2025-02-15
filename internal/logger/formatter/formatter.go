package formatter

import "github.com/hueristiq/xurlfind3r/internal/logger/levels"

type Log struct {
	Message  string
	Level    levels.Level
	Metadata map[string]string
}

type Formatter interface {
	Format(log *Log) (data []byte, err error)
}
