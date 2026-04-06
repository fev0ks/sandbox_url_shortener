package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"sandbox_url_shortener/internal/platform/logger"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging attaches a request-scoped logger to context and logs one summary line per request.
func Logging(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			requestID := GetRequestID(r.Context())

			reqLogger := base.With(
				slog.String("request_id", requestID),
			)

			ctx := logger.WithContext(r.Context(), reqLogger)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(sw, r)

			reqLogger.InfoContext(ctx, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status_code", sw.code),
				slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000.0),
				slog.String("client_ip", r.RemoteAddr),
			)
		})
	}
}
