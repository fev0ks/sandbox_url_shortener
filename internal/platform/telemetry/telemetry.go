package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/platform/requestmeta"
)

type Config struct {
	ServiceName  string
	Environment  string
	OTLPEndpoint string
}

// InitTracing configures the global tracer provider. When no endpoint is set,
// tracing stays in noop mode and the returned shutdown is also a noop.
func InitTracing(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	endpoint := sanitizeEndpoint(cfg.OTLPEndpoint)
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create OTLP HTTP trace exporter: %w", err)
	}

	resource, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build telemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
	)
	otel.SetTracerProvider(provider)

	return provider.Shutdown, nil
}

// HTTPMiddleware creates one server span per incoming request.
func HTTPMiddleware(service string) middleware.Middleware {
	tracer := otel.Tracer(service + ".http")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipHTTPObservability(r) {
				next.ServeHTTP(w, r)
				return
			}

			initialRoute := httpRouteLabel(r)
			ctx, span := tracer.Start(r.Context(), initialRoute, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
			defer span.End()

			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", initialRoute),
				attribute.String("http.target", r.URL.Path),
			)

			if requestID := requestmeta.RequestID(ctx); requestID != "" {
				span.SetAttributes(attribute.String("request.id", requestID))
			}
			if correlationID := requestmeta.CorrelationID(ctx); correlationID != "" {
				span.SetAttributes(attribute.String("correlation.id", correlationID))
			}

			if sc := span.SpanContext(); sc.IsValid() {
				ctx = requestmeta.WithTraceID(ctx, sc.TraceID().String())
			}

			req := r.WithContext(ctx)
			rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(rec, req)

			finalRoute := httpRouteLabel(req)
			span.SetName(finalRoute)
			span.SetAttributes(attribute.String("http.route", finalRoute))
			span.SetAttributes(attribute.Int("http.status_code", rec.code))
			if rec.code >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(rec.code))
				return
			}
			span.SetStatus(codes.Ok, "")
		})
	}
}

func sanitizeEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	return strings.TrimSuffix(raw, "/")
}

func httpRouteLabel(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if pattern := r.Pattern; pattern != "" {
		return pattern
	}
	return r.Method + " unmatched"
}

func shouldSkipHTTPObservability(r *http.Request) bool {
	if r == nil {
		return false
	}
	return r.URL.Path == "/metrics" || strings.HasPrefix(r.URL.Path, "/health/")
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (w *statusRecorder) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
