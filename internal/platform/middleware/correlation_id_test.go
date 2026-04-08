package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrelationID_UsesHeaderWhenPresent(t *testing.T) {
	handler := RequestID(CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetCorrelationID(r.Context()); got != "corr-123" {
			t.Errorf("correlation_id = %q, want %q", got, "corr-123")
		}
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerCorrelationID, "corr-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(headerCorrelationID); got != "corr-123" {
		t.Errorf("response correlation header = %q, want %q", got, "corr-123")
	}
}

func TestCorrelationID_FallsBackToRequestID(t *testing.T) {
	handler := RequestID(CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetCorrelationID(r.Context()); got != "req-123" {
			t.Errorf("correlation_id = %q, want %q", got, "req-123")
		}
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerRequestID, "req-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(headerCorrelationID); got != "req-123" {
		t.Errorf("response correlation header = %q, want %q", got, "req-123")
	}
}
