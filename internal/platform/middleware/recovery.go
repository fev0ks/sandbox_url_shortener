package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery catches panics, logs the stack trace, and returns a 500 JSON error.
// Takes the base logger explicitly (same shape as Logging) so panic logs go
// through the context-aware handler and pick up request_id/correlation_id.
func Recovery(base *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			defer func(ctx context.Context) {
				if rec := recover(); rec != nil {
					base.ErrorContext(ctx, "panic recovered",
						"operation", "http_recovery",
						"error_kind", "panic",
						"panic", fmt.Sprintf("%v", rec),
						"stack", string(debug.Stack()),
					)

					requestID := GetRequestID(ctx)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					if err := json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]string{
							"code":       "internal_error",
							"message":    "internal server error",
							"request_id": requestID,
						},
					}); err != nil {
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					}
				}
			}(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
