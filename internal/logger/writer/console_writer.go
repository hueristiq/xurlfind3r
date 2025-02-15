package writer

import (
	"os"
	"sync"

	"github.com/hueristiq/xurlfind3r/internal/logger/levels"
)

type Console struct {
	mutex *sync.Mutex
}

func (c *Console) Write(data []byte, level levels.Level) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	//nolint: exhaustive
	switch level {
	case levels.LevelSilent:
		os.Stdout.Write(data)
		os.Stdout.WriteString("\n")
	default:
		os.Stderr.Write(data)
		os.Stderr.WriteString("\n")
	}
}

var _ Writer = (*Console)(nil)

func NewConsoleWriter() (writer *Console) {
	writer = &Console{
		mutex: &sync.Mutex{},
	}

	return
}
