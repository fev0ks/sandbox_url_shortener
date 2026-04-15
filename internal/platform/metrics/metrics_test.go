package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMiddleware_SkipsHealthAndMetricsRoutes(t *testing.T) {
	handler := HTTPMiddleware("metrics-test-skip")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	skippedPaths := []string{"/health/live", "/health/ready", "/metrics"}
	for _, path := range skippedPaths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("metrics-test-skip", http.MethodGet, "GET unmatched", "200")); got != 0 {
		t.Fatalf("http_requests_total = %v, want 0 for skipped routes", got)
	}
}

func TestHTTPMiddleware_RecordsApplicationRequest(t *testing.T) {
	handler := HTTPMiddleware("metrics-test-app")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Pattern = "POST /api/v1/links"
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("metrics-test-app", http.MethodPost, "POST /api/v1/links", "201")); got != 1 {
		t.Fatalf("http_requests_total = %v, want 1", got)
	}
}
