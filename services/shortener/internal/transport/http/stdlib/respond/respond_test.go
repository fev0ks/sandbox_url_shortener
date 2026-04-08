package respond

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sandbox_url_shortener/internal/platform/requestmeta"
)

func TestError_JSONShape(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx := requestmeta.WithRequestID(context.Background(), "req-123")
	Error(ctx, rec, http.StatusNotFound, "link_not_found", "short code not found")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "link_not_found" {
		t.Errorf("code = %q, want %q", body.Error.Code, "link_not_found")
	}
	if body.Error.RequestID != "req-123" {
		t.Errorf("request_id = %q, want %q", body.Error.RequestID, "req-123")
	}
}

func TestJSON_Success(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}
	JSON(rec, http.StatusOK, data)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body[status] = %q, want %q", body["status"], "ok")
	}
}
