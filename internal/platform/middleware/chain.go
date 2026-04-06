package middleware

import (
	"log/slog"
	"net/http"
)

// Chain applies the standard middleware stack in the correct order:
// recovery -> request_id -> logging -> handler
func Chain(base *slog.Logger, handler http.Handler) http.Handler {
	return Recovery(
		RequestID(
			Logging(base)(handler),
		),
	)
}
