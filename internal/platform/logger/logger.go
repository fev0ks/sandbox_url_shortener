// Package logger provides a configured slog.Logger with JSON output to stdout.
//
// The logger is wrapped in a context-aware slog.Handler (ctxHandler) that
// automatically enriches every record with request-scoped metadata
// (request_id, correlation_id, trace_id) pulled from the context passed to
// slog.*Context calls. Call sites do not need to manually attach those fields.
//
// Usage:
//
//	log := logger.New("INFO", "shortener", "dev")
//	log.InfoContext(ctx, "link created", "link_id", id)
//
// As long as the request middleware has populated requestmeta in ctx,
// request_id / correlation_id / trace_id appear in the JSON automatically.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"

	"sandbox_url_shortener/internal/platform/requestmeta"
)

// New creates a slog.Logger with JSON output to stdout, base attributes, and the
// given level. The returned logger automatically enriches records from context.
func New(level, service, env string) *slog.Logger {
	return NewWithWriter(os.Stdout, level, service, env)
}

// NewWithWriter behaves like New but sends output to the given writer.
//
// Use it when stdout is reserved for something else. A process speaking a
// line-oriented protocol on stdout (for example an MCP server on stdio) must
// log to os.Stderr, otherwise log records corrupt the protocol stream.
func NewWithWriter(w io.Writer, level, service, env string) *slog.Logger {
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

	base := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})

	return slog.New(&ctxHandler{Handler: base}).With(
		slog.String("service", service),
		slog.String("env", env),
	)
}

// ctxHandler decorates an slog.Handler so that every record is enriched with
// request-scoped attributes pulled from context. Enrichment runs only after
// the underlying handler accepts the record's level, so filtered logs pay no cost.
type ctxHandler struct {
	slog.Handler
}

func (h *ctxHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs := requestmeta.AttrsFromContext(ctx); len(attrs) > 0 {
		r.AddAttrs(attrs...)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ctxHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ctxHandler) WithGroup(name string) slog.Handler {
	return &ctxHandler{Handler: h.Handler.WithGroup(name)}
}
