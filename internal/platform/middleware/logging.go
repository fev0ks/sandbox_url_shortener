package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"sandbox_url_shortener/internal/platform/httppolicy"
)

// defaultLogSkipPaths are noisy infra endpoints we don't want in access logs.
// Probes and scrapers hit them every few seconds and drown out real traffic.
var defaultLogSkipPaths = map[string]struct{}{
	"/health/live":  {},
	"/health/ready": {},
	"/metrics":      {},
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// Logging emits one structured line per request, skipping infra paths
// (health probes, /metrics) so access logs stay readable.
//
// Errors and 5xx responses on skipped paths are still logged so failing
// probes are visible.
func Logging(base *slog.Logger) func(http.Handler) http.Handler {
	return LoggingWithSkip(base, defaultLogSkipPaths)
}

// LoggingWithSkip is like Logging but lets the caller override the skip set.
// Pass nil to log every request.
func LoggingWithSkip(base *slog.Logger, skip map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
			next.ServeHTTP(sw, r)

			if _, skipped := skip[r.URL.Path]; skipped && sw.code < 500 {
				return
			}

			base.InfoContext(ctx, "request",
				slog.String("operation", "http_request"),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status_code", sw.code),
				slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000.0),
				slog.String("client_ip", httppolicy.ClientIP(r)),
				slog.String("user_agent", r.UserAgent()),
			)
		})
	}
}
