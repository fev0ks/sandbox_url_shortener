package httppolicy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testRateLimitNamespace = "ratelimit:create"

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.5")
	if got := ClientIP(req); got != "203.0.113.1" {
		t.Errorf("got %q", got)
	}
}

func TestClientIP_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:55555"
	if got := ClientIP(req); got != "10.0.0.1" {
		t.Errorf("got %q", got)
	}
}

func TestClientIPKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:55555"
	if got := ClientIPKey(testRateLimitNamespace, req); got != "ratelimit:create:10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := ClientIPKey("", req); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteRateLimited(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRateLimited(rec, "req-1", 42*time.Second)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "42" {
		t.Errorf("Retry-After = %q", got)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestWriteRateLimited_ClampsToOne(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRateLimited(rec, "req-1", 100*time.Millisecond)
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("got %q", got)
	}
}
