package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("expected request_id in context, got empty")
		}
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get(headerRequestID) == "" {
		t.Error("expected X-Request-ID in response header")
	}
}

func TestRequestID_ReusesExisting(t *testing.T) {
	const existing = "test-request-123"

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id != existing {
			t.Errorf("request_id = %q, want %q", id, existing)
		}
	})

	handler := RequestID(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerRequestID, existing)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get(headerRequestID); got != existing {
		t.Errorf("response X-Request-ID = %q, want %q", got, existing)
	}
}
