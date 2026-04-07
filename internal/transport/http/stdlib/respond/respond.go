// Package respond provides JSON response helpers for the HTTP transport layer.
package respond

import (
	"encoding/json"
	"net/http"
)

// errorBody is the standard error response shape.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Error writes a JSON error response with the standard shape.
func Error(w http.ResponseWriter, statusCode int, code, message, requestID string) {
	body, err := json.Marshal(errorBody{
		Error: errorDetail{
			Code:      code,
			Message:   message,
			RequestID: requestID,
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
