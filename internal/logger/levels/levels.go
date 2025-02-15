package levels

type Level int

func (l Level) String() string {
	return [...]string{"fatal", "silent", "error", "info", "warn", "debug"}[l]
}

const (
	LevelFatal Level = iota
	LevelSilent
	LevelError
	LevelInfo
	LevelWarn
	LevelDebug
)
