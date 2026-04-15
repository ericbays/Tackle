// Package logger provides structured logging setup for the Tackle platform.
package logger

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"

	"tackle/internal/services/applog"
)

// OmnibusHandler wraps a base handler and dynamically forwards to AppLogService.
type OmnibusHandler struct {
	base     slog.Handler
	appSvc   atomic.Pointer[applog.AppLogService]
}

func (h *OmnibusHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *OmnibusHandler) Handle(ctx context.Context, r slog.Record) error {
	// Let the base handler (os.Stdout) process it first.
	err := h.base.Handle(ctx, r)

	// In the background, push to the AppLog hook if connected.
	if svc := h.appSvc.Load(); svc != nil {
		attrMap := make(map[string]any)
		r.Attrs(func(a slog.Attr) bool {
			// Resolve any lazy log values
			val := a.Value.Resolve().Any()
			attrMap[a.Key] = val
			return true
		})

		entry := applog.AppLogEntry{
			Timestamp:  r.Time,
			Level:      r.Level.String(),
			Message:    r.Message,
			Attributes: attrMap,
		}
		
		svc.Log(entry)
	}

	return err
}

func (h *OmnibusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &OmnibusHandler{
		base:   h.base.WithAttrs(attrs),
	}
}

func (h *OmnibusHandler) WithGroup(name string) slog.Handler {
	return &OmnibusHandler{
		base:   h.base.WithGroup(name),
	}
}

// HookAppLogService attaches the asynchronous database writer to the logger.
func (h *OmnibusHandler) HookAppLogService(svc *applog.AppLogService) {
	h.appSvc.Store(svc)
}

// New returns a configured *slog.Logger.
// In development, it uses a human-readable text handler.
// In production, it uses a JSON handler.
func New(isDevelopment bool) (*slog.Logger, *OmnibusHandler) {
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

	omnibus := &OmnibusHandler{base: handler}
	return slog.New(omnibus), omnibus
}
