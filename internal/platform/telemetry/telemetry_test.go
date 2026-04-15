package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPMiddleware_SkipsHealthAndMetricsRoutes(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(prev)

	handler := HTTPMiddleware("telemetry-test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/health/live", "/health/ready", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	spans := recorder.Ended()
	if len(spans) != 0 {
		t.Fatalf("ended spans = %d, want 0 for skipped routes", len(spans))
	}
}

func TestHTTPMiddleware_UsesResolvedRouteName(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := trace.NewTracerProvider(trace.WithSpanProcessor(recorder))

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(prev)

	handler := HTTPMiddleware("telemetry-test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Pattern = "POST /api/v1/links"
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if got := spans[0].Name(); got != "POST /api/v1/links" {
		t.Fatalf("span name = %q, want %q", got, "POST /api/v1/links")
	}

	attrs := spans[0].Attributes()
	var route string
	var statusCode int64
	for _, attr := range attrs {
		switch string(attr.Key) {
		case "http.route":
			route = attr.Value.AsString()
		case "http.status_code":
			statusCode = attr.Value.AsInt64()
		}
	}
	if route != "POST /api/v1/links" {
		t.Fatalf("http.route = %q, want %q", route, "POST /api/v1/links")
	}
	if statusCode != http.StatusCreated {
		t.Fatalf("http.status_code = %d, want %d", statusCode, http.StatusCreated)
	}
}

func TestInitTracing_NoEndpointIsNoop(t *testing.T) {
	shutdown, err := InitTracing(context.Background(), Config{
		ServiceName: "telemetry-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("InitTracing error: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}
