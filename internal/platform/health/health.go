// Package health provides shared liveness and readiness HTTP handlers.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type response struct {
	Status string `json:"status"`
}

// Checker is a function that checks a dependency's health.
type Checker func(ctx context.Context) error

// LiveHandler returns 200 OK with {"status": "ok"} for liveness probes.
func LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	}
}

// ReadyHandler returns 200 when all checks pass, 503 if any fails.
// If no checks are provided it always returns 200.
func ReadyHandler(checks ...Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		for _, check := range checks {
			if err := check(ctx); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, response{Status: "unavailable"})
				return
			}
		}
		writeJSON(w, http.StatusOK, response{Status: "ok"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write(body); err != nil {
		return
	}
}
