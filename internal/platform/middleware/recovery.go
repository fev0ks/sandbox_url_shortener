package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/transport/http/stdlib/respond"
)

// Recovery catches panics, logs the stack trace, and returns a 500 JSON error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				l := logger.FromContext(r.Context())
				l.ErrorContext(r.Context(), "panic recovered",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)

				requestID := GetRequestID(r.Context())
				respond.Error(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
