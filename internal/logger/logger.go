package logger

import (
	"fmt"
	"os"
	"strings"

	"github.com/hueristiq/xurlfind3r/internal/logger/formatter"
	"github.com/hueristiq/xurlfind3r/internal/logger/levels"
	"github.com/hueristiq/xurlfind3r/internal/logger/writer"
)

type Logger struct {
	formatter   formatter.Formatter
	writer      writer.Writer
	maxLogLevel levels.Level
}

func (l *Logger) SetFormatter(f formatter.Formatter) {
	l.formatter = f
}

func (l *Logger) SetWriter(w writer.Writer) {
	l.writer = w
}

func (l *Logger) SetMaxLogLevel(level levels.Level) {
	l.maxLogLevel = level
}

func (l *Logger) Log(event *Event) {
	if event.level > l.maxLogLevel {
		return
	}

	var (
		ok    bool
		label string
	)

	if _, ok = event.metadata["label"]; !ok {
		labels := map[levels.Level]string{
			levels.LevelFatal: "FTL",
			levels.LevelError: "ERR",
			levels.LevelInfo:  "INF",
			levels.LevelWarn:  "WRN",
			levels.LevelDebug: "DBG",
		}

		if label, ok = labels[event.level]; ok {
			event.metadata["label"] = label
		}
	}

	event.message = strings.TrimSuffix(event.message, "\n")

	data, err := l.formatter.Format(&formatter.Log{
		Message:  event.message,
		Level:    event.level,
		Metadata: event.metadata,
	})
	if err != nil {
		return
	}

	l.writer.Write(data, event.level)

	if event.level == levels.LevelFatal {
		os.Exit(1)
	}
}

func (l *Logger) Fatal() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelFatal,
		metadata: make(map[string]string),
	}

	return
}

func (l *Logger) Print() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelSilent,
		metadata: make(map[string]string),
	}

	return
}

func (l *Logger) Error() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelError,
		metadata: make(map[string]string),
	}

	return
}

func (l *Logger) Info() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelInfo,
		metadata: make(map[string]string),
	}

	return
}

func (l *Logger) Warn() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelWarn,
		metadata: make(map[string]string),
	}

	return
}

func (l *Logger) Debug() (event *Event) {
	event = &Event{
		logger:   l,
		level:    levels.LevelDebug,
		metadata: make(map[string]string),
	}

	return
}

type Event struct {
	logger   *Logger
	level    levels.Level
	message  string
	metadata map[string]string
}

func (e *Event) Label(label string) (event *Event) {
	e.metadata["label"] = label

	return e
}

func (e *Event) Msg(message string) {
	e.message = message

	e.logger.Log(e)
}

func (e *Event) Msgf(format string, args ...interface{}) {
	e.message = fmt.Sprintf(format, args...)

	e.logger.Log(e)
}

var DefaultLogger *Logger

func init() {
	DefaultLogger = &Logger{}

	DefaultLogger.SetFormatter(formatter.NewConsoleFormatter(&formatter.ConsoleFormatterConfiguration{
		Colorize: true,
	}))
	DefaultLogger.SetWriter(writer.NewConsoleWriter())
	DefaultLogger.SetMaxLogLevel(levels.LevelDebug)
}

func Fatal() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelFatal,
		metadata: make(map[string]string),
	}

	return
}

func Print() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelSilent,
		metadata: make(map[string]string),
	}

	return
}

func Error() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelError,
		metadata: make(map[string]string),
	}

	return
}

func Info() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelInfo,
		metadata: make(map[string]string),
	}

	return
}

func Warn() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelWarn,
		metadata: make(map[string]string),
	}

	return
}

func Debug() (event *Event) {
	event = &Event{
		logger:   DefaultLogger,
		level:    levels.LevelDebug,
		metadata: make(map[string]string),
	}

	return
}
