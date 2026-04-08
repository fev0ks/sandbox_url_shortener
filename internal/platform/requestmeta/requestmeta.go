// Package requestmeta stores request-scoped metadata in context.
package requestmeta

import (
	"context"
	"log/slog"
)

type (
	requestIDKey     struct{}
	correlationIDKey struct{}
	traceIDKey       struct{}
)

// WithRequestID stores request_id in context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID extracts request_id from context.
func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithCorrelationID stores correlation_id in context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// CorrelationID extracts correlation_id from context.
func CorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithTraceID stores trace_id in context.
// This is reserved for future OTel or traceparent extraction wiring.
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceID extracts trace_id from context.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(traceIDKey{}).(string); ok {
		return id
	}
	return ""
}

// AttrsFromContext returns structured logging attributes for known request metadata.
// Returns nil when there's nothing in context, so callers can cheaply skip work.
func AttrsFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}

	rid := RequestID(ctx)
	cid := CorrelationID(ctx)
	tid := TraceID(ctx)
	if rid == "" && cid == "" && tid == "" {
		return nil
	}

	attrs := make([]slog.Attr, 0, 3)
	if rid != "" {
		attrs = append(attrs, slog.String("request_id", rid))
	}
	if cid != "" {
		attrs = append(attrs, slog.String("correlation_id", cid))
	}
	if tid != "" {
		attrs = append(attrs, slog.String("trace_id", tid))
	}
	return attrs
}
