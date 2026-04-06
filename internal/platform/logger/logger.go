// Package logger provides a configured slog.Logger with JSON output to stdout.
package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New creates a slog.Logger with JSON output, base attributes, and the given level.
func New(level, service, env string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "DEBUG":
		lvl = slog.LevelDebug
	case "WARN":
		lvl = slog.LevelWarn
	case "ERROR":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	})

	return slog.New(handler).With(
		slog.String("service", service),
		slog.String("env", env),
	)
}

// WithContext stores the logger in context.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the logger from context. Falls back to slog.Default.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
