// Package respond provides JSON response helpers for the identity HTTP transport layer.
package respond

import (
	"context"
	"encoding/json"
	"net/http"

	"sandbox_url_shortener/internal/platform/middleware"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Error writes a JSON error response with the standard shape.
func Error(ctx context.Context, w http.ResponseWriter, statusCode int, code, message string) {
	body, err := json.Marshal(errorBody{
		Error: errorDetail{
			Code:      code,
			Message:   message,
			RequestID: middleware.GetRequestID(ctx),
		},
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(body); err != nil {
		return
	}
}

// JSON writes a JSON success response.
func JSON(w http.ResponseWriter, statusCode int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if _, err := w.Write(body); err != nil {
			return
		}
	}
}
