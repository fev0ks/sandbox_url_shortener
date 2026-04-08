// Package httppolicy provides small, composable HTTP policy primitives
// (rate limiting, client-IP extraction, standard 429 response) that feature
// handlers can call inline without adopting a middleware stack.
package httppolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Decision is the result of one limiter check.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter is the narrow contract consumed by callers that want to enforce
// a rate limit. Implementations live in the repository layer (e.g., Redis).
type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

// ClientIPKey returns "namespace:ip" for use as a limiter key.
func ClientIPKey(namespace string, r *http.Request) string {
	ip := ClientIP(r)
	if namespace == "" {
		return ip
	}
	return fmt.Sprintf("%s:%s", namespace, ip)
}

// ClientIP extracts the best-effort client IP. Prefers the first segment of
// X-Forwarded-For when present, otherwise falls back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// WriteRateLimited writes the standard 429 envelope with a Retry-After header.
// retryAfter is clamped to at least 1 second.
func WriteRateLimited(w http.ResponseWriter, requestID string, retryAfter time.Duration) {
	secs := int(retryAfter.Seconds())
	if secs <= 0 {
		secs = 1
	}
	body := map[string]any{
		"error": map[string]any{
			"code":       "rate_limited",
			"message":    "rate limit exceeded",
			"request_id": requestID,
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	w.WriteHeader(http.StatusTooManyRequests)
	if _, err := w.Write(payload); err != nil {
		return
	}
}
