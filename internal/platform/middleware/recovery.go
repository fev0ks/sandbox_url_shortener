package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/transport/http/stdlib/respond"
)

// Recovery catches panics, logs the stack trace, and returns a 500 JSON error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		defer func(ctx context.Context) {
			if rec := recover(); rec != nil {
				l := logger.FromContext(ctx)
				l.ErrorContext(ctx, "panic recovered",
					"panic", fmt.Sprintf("%v", rec),
					"stack", string(debug.Stack()),
				)

				requestID := GetRequestID(ctx)
				respond.Error(w, http.StatusInternalServerError, "internal_error", "internal server error", requestID)
			}
		}(ctx)
		next.ServeHTTP(w, r)
	})
}
