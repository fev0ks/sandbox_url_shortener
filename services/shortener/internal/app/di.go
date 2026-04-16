package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"sandbox_url_shortener/services/shortener/internal/config"
	kafkarepo "sandbox_url_shortener/services/shortener/internal/repository/kafka"
	"sandbox_url_shortener/services/shortener/internal/repository/postgres"
	redisrepo "sandbox_url_shortener/services/shortener/internal/repository/redis"
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
	linkRepo         *postgres.LinkRepo
	linkCache        *redisrepo.LinkCache
	rateLimiter      *redisrepo.FixedWindowLimiter
	idempotencyStore *redisrepo.IdempotencyStore
	visitPublisher   service.LinkVisitPublisher
	visitCloser      interface{ Close() error }

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
func newContainer(ctx context.Context, cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool, redisClient *goredis.Client) *container {
	c := &container{}

	// Repositories.
	c.linkRepo = postgres.NewLinkRepo(pool)
	c.linkCache = redisrepo.NewLinkCache(redisClient)
	c.rateLimiter = redisrepo.NewFixedWindowLimiter(
		redisClient,
		cfg.RateLimitPerMin,
		time.Minute,
	)
	c.idempotencyStore = redisrepo.NewIdempotencyStore(redisClient)
	if cfg.KafkaProducerEnabled {
		basePublisher := kafkarepo.NewLinkVisitPublisher(splitBrokers(cfg.KafkaBrokers), cfg.KafkaTopicLinkVisited)
		asyncPublisher := newAsyncLinkVisitPublisher(ctx, log, basePublisher)
		c.visitPublisher = asyncPublisher
		c.visitCloser = asyncPublisher
	}

	// Services.
	c.linkSvc = service.NewLinkService(c.linkRepo, c.linkCache, c.visitPublisher, log)

	// HTTP feature handlers.
	c.linkHandler = link.NewHandler(
		c.linkSvc,
		log,
		cfg.BaseURL,
		c.idempotencyStore,
		c.rateLimiter,
	)
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

func splitBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		if broker := strings.TrimSpace(part); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}
