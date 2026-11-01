package main

import (
	"context"
	"log/slog"
	"os"

	hqgologger "github.com/hueristiq/hq-lib-logger-go"
)

// fatal logs at the fatal level and exits, restoring the old logger.Fatal
// contract (the updated logger never exits by itself).
func fatal(l *slog.Logger, msg string, fields ...any) {
	l.Log(context.Background(), hqgologger.SlogLevelFatal, msg, fields...)
	os.Exit(1)
}
