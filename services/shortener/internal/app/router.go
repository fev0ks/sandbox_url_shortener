package app

import (
	"context"
	"net/http"

	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/platform/telemetry"
)

// buildRouter constructs the full HTTP handler tree for the shortener binary.
//
// This file is the table of contents for the binary's HTTP surface — every
// route is either registered directly here (platform routes) or via a feature
// server's Register method. Add a new feature by writing its package under
// internal/transport/http/stdlib/<feature>/, wiring it in di.go, and adding
// one Register call below.
func (a *App) buildRouter() http.Handler {
	mux := http.NewServeMux()

	// Platform routes — owned by the binary itself.
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler(func(ctx context.Context) error {
		return a.pool.Ping(ctx)
	}))
	mux.Handle("GET /metrics", metrics.Handler())

	// Feature handlers — each owns its own routes.
	a.di.linkHandler.Register(mux)
	a.di.redirectHandler.Register(mux)

	// Cross-cutting middleware — first entry runs first at request time.
	return middleware.Compose(mux,
		middleware.Recovery(a.log),            // 1. catch panics first
		middleware.RequestID,                  // 2. assign request_id before anything reads it
		middleware.CorrelationID,              // 3. derives from request_id when header missing
		telemetry.HTTPMiddleware("shortener"), // 4. create trace/span context before logs
		metrics.HTTPMiddleware("shortener"),   // 5. emit request metrics after handler completes
		middleware.Logging(a.log),             // 6. logs last so status_code is final
	)
}
