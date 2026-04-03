// Package logger provides structured logging setup for the Tackle platform.
package logger

import (
	"log/slog"
	"os"
)

// New returns a configured *slog.Logger.
// In development, it uses a human-readable text handler.
// In production, it uses a JSON handler.
func New(isDevelopment bool) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename the default time key to "timestamp" for consistency.
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	}

	if isDevelopment {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
