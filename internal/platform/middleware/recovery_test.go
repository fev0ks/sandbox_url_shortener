package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sandbox_url_shortener/internal/platform/requestmeta"
)

// newTestLogger returns a logger that writes JSON records into buf,
// so tests can assert on the structured fields the middleware emitted.
func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestRecovery_PassesThroughWhenNoPanic(t *testing.T) {
	var buf bytes.Buffer
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})

	handler := Recovery(newTestLogger(&buf))(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no logs on happy path, got: %s", buf.String())
	}
}

func TestRecovery_RecoversFromPanic(t *testing.T) {
	var buf bytes.Buffer
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	handler := Recovery(newTestLogger(&buf))(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links", nil)
	// Seed a request_id in context the same way RequestID middleware would,
	// so we can verify the JSON error body echoes it back to the client.
	req = req.WithContext(requestmeta.WithRequestID(req.Context(), "req-123"))
	rec := httptest.NewRecorder()

	// The handler must NOT propagate the panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic leaked through Recovery: %v", r)
		}
	}()
	handler.ServeHTTP(rec, req)

	// 500 with the standard JSON error envelope.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("error.code = %q, want internal_error", body.Error.Code)
	}
	if body.Error.RequestID != "req-123" {
		t.Errorf("error.request_id = %q, want req-123", body.Error.RequestID)
	}

	// The recovered panic must be logged with msg, panic value, and a stack trace.
	logs := buf.String()
	if !strings.Contains(logs, `"msg":"panic recovered"`) {
		t.Errorf("expected 'panic recovered' log, got: %s", logs)
	}
	if !strings.Contains(logs, `"panic":"boom"`) {
		t.Errorf("expected panic value in log, got: %s", logs)
	}
	if !strings.Contains(logs, `"stack":`) {
		t.Errorf("expected stack trace in log, got: %s", logs)
	}
}

func TestRecovery_RecoversFromNonStringPanic(t *testing.T) {
	var buf bytes.Buffer
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Panics aren't always strings — make sure %v formatting handles it.
		panic(struct{ N int }{N: 42})
	})

	handler := Recovery(newTestLogger(&buf))(inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(buf.String(), `"panic":"{42}"`) {
		t.Errorf("expected struct panic to be %%v-formatted, got: %s", buf.String())
	}
}
