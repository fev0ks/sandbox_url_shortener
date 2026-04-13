package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogging_EmitsStandardAccessFields(t *testing.T) {
	var buf bytes.Buffer

	handler := Logging(newTestLogger(&buf))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", nil)
	req.Header.Set("User-Agent", "E07 logging test")
	req.RemoteAddr = "203.0.113.10:4321"

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}

	if got := entry["msg"]; got != "request" {
		t.Fatalf("msg = %v, want request", got)
	}
	if got := entry["operation"]; got != "http_request" {
		t.Fatalf("operation = %v, want http_request", got)
	}
	if got := entry["method"]; got != http.MethodPost {
		t.Fatalf("method = %v, want %s", got, http.MethodPost)
	}
	if got := entry["path"]; got != "/api/v1/links" {
		t.Fatalf("path = %v, want /api/v1/links", got)
	}
	if got := entry["status_code"]; got != float64(http.StatusCreated) {
		t.Fatalf("status_code = %v, want %d", got, http.StatusCreated)
	}
	if got := entry["client_ip"]; got != "203.0.113.10" {
		t.Fatalf("client_ip = %v, want 203.0.113.10", got)
	}
	if got := entry["user_agent"]; got != "E07 logging test" {
		t.Fatalf("user_agent = %v, want E07 logging test", got)
	}
	if _, ok := entry["latency_ms"]; !ok {
		t.Fatal("latency_ms missing from access log")
	}
}
