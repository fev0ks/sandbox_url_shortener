package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"sandbox_url_shortener/internal/platform/requestmeta"
)

const headerRequestID = "X-Request-ID"

// RequestID reads X-Request-ID from the request or generates a new one,
// stores it in context, and sets it on the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = generateID()
		}

		w.Header().Set(headerRequestID, id)

		ctx := requestmeta.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context.
func GetRequestID(ctx context.Context) string {
	return requestmeta.RequestID(ctx)
}

func withCorrelationID(ctx context.Context, id string) context.Context {
	return requestmeta.WithCorrelationID(ctx, id)
}

// GetCorrelationID extracts the correlation ID from context.
func GetCorrelationID(ctx context.Context) string {
	return requestmeta.CorrelationID(ctx)
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
