package app

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/services/shortener/internal/config"
	"sandbox_url_shortener/services/shortener/internal/repository/postgres"
	"sandbox_url_shortener/services/shortener/internal/service"
	"sandbox_url_shortener/services/shortener/internal/transport/http/stdlib/link"
	"sandbox_url_shortener/services/shortener/internal/transport/http/stdlib/redirect"
)

// container holds every wired component for the shortener binary,
// grouped by architectural layer. It is built once at startup by newContainer.
//
// Why a struct instead of lazy getters:
//   - eager construction fails fast at boot, not on first request
//   - all errors surface through NewApp (and ultimately main), never via os.Exit
//   - shutdown order is controlled explicitly through App's LIFO closer chain
//
// How to add a new component:
//  1. add a field below in the correct layer block
//  2. construct it inside newContainer in the same layer block
//  3. reference it from initHTTPServer (or wherever it is needed)
type container struct {
	// Repositories — store-specific data access.
	linkRepo *postgres.LinkRepo

	// Services — business logic.
	linkSvc *service.LinkService

	// HTTP feature handlers — one package per feature under transport/http/stdlib/.
	linkHandler     *link.Handler
	redirectHandler *redirect.Handler
}

// newContainer wires the full dependency graph for the API binary.
//
// It returns an error so future components that can fail at construction time
// (Redis client, Kafka producer, ClickHouse pool, third-party SDKs, ...) can
// surface that failure cleanly through NewApp instead of panicking.
func newContainer(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) *container {
	c := &container{}

	// Repositories.
	c.linkRepo = postgres.NewLinkRepo(pool)

	// Services.
	c.linkSvc = service.NewLinkService(c.linkRepo, log)

	// HTTP feature handlers.
	c.linkHandler = link.NewHandler(c.linkSvc, log, cfg.BaseURL)
	c.redirectHandler = redirect.NewHandler(c.linkSvc, log)

	return c
}

// wrapInitErr is a small helper for future constructors that may fail.
// Keeping it here so each new init line stays short and consistent.
//
//nolint:unused // reserved for future fallible constructors
func wrapInitErr(component string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("container: init %s: %w", component, err)
}
