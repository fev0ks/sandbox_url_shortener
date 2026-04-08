# E03-001 - Add Redis Cache-Aside To Redirect Flow

Status: complete

Epic: E03 - Redirect Performance And Safety

## Summary

Introduce a Redis cache-aside layer in front of the redirect lookup. On cache hit serve the
redirect from cached data while still updating the authoritative PostgreSQL redirect counter
for unlimited links (`max_clicks = 0`); on miss fall back to PostgreSQL, warm the cache, and
serve. Cache failures must degrade gracefully — Redis being down should not break redirects.

## Scope

- new Redis client wired through the App composition root; boot connectivity is required, but
  readiness stays PostgreSQL-only because Redis is treated as a fail-open dependency
- new `LinkCache` interface in `services/shortener/internal/domain/` and Redis implementation
  under `services/shortener/internal/repository/redis/`
- update `LinkService.Resolve` to consult the cache before calling the repository
- cache-warm-up after a PostgreSQL miss in the resolve path
- preserve `max_clicks` semantics: links with `max_clicks > 0` always confirm against
  PostgreSQL even on cache hit
- excludes cache invalidation on disable/enable/delete (covered by E03-002)
- excludes cache metrics dashboards (E08)

## Acceptance Criteria

- Redis client is created in `initContainer` with `REDIS_ADDR` config; failure to connect at
  boot returns an error from `NewApp`
- redirect happy path on cache hit returns 302 without a PostgreSQL read query when
  `max_clicks = 0`, while still keeping `redirect_count` authoritative in PostgreSQL
- redirect on cache miss falls back to PostgreSQL, populates the cache, and returns 302
- redirect when Redis is down logs a warning once per request and falls back to PostgreSQL
  successfully
- cached value includes `original_url`, `max_clicks`, and `expires_at`
- cache key format is `link:code:{short_code}`
- TTL is `min(24h, time_until_expires_at)`; if `expires_at` is nil the TTL is 24h
- `LinkCache` interface lives in `services/shortener/internal/domain/`; the service depends on the
  interface,
  not on the Redis client
- unit tests for the service path with a mock cache cover: hit (no DB), miss (DB + warm),
  cache error (DB only)
- `go build`, `go vet`, `go test ./...` all pass

## Assumptions

- Redis is already running via docker-compose (added in E01)
- the project does not need cluster mode or sentinel — single-node Redis is enough
- `redis/go-redis/v9` is the chosen client library
- cache failures should never propagate as 5xx — they degrade to a slower-but-correct PG path
- `redirect_count` remains authoritative in PostgreSQL; the cache does not store it

## Implementation Notes

- add `REDIS_ADDR` (default `localhost:6379`) and `REDIS_DB` (default `0`) to
  `services/shortener/internal/config`
- new file `services/shortener/internal/repository/redis/link_cache.go` implementing `domain.LinkCache`:

  ```go
  type LinkCache interface {
      Get(ctx context.Context, shortCode string) (*CachedLink, error) // returns nil, nil on miss
      Set(ctx context.Context, shortCode string, c *CachedLink, ttl time.Duration) error
      Delete(ctx context.Context, shortCode string) error
  }

  type CachedLink struct {
      OriginalURL string
      MaxClicks   int64
      ExpiresAt   *time.Time
  }
  ```

- service algorithm in `Resolve`:
  1. Try `cache.Get(shortCode)`. On error: log warn, treat as miss.
  2. If hit and `MaxClicks == 0` and not expired → call lightweight
     `repo.TouchRedirect(shortCode)` to keep PostgreSQL authoritative, then return synthesized
     `domain.Link`.
  3. Otherwise call `repo.ResolveAndIncrement` (existing path).
  4. On PG success: best-effort `cache.Set` with computed TTL. Failures swallowed and logged.
- TTL helper: `func computeTTL(expiresAt *time.Time, now time.Time) time.Duration` returning
  the smaller of 24h and `expiresAt.Sub(now)`, with a floor of zero (skip cache write).
- container wiring:
  - new field `redis *redis.Client`
  - new field `linkCache *redisrepo.LinkCache`
  - service constructor takes both `repo` and `cache`
- closer registered for the Redis client in `initRedis()`
- redirect handler is unchanged — all logic stays in the service

### Files changed

| File | Change |
|------|--------|
| `services/shortener/internal/config/config.go` | add `RedisAddr`, `RedisDB` |
| `services/shortener/internal/domain/link_cache.go` | new — LinkCache interface + CachedLink struct |
| `services/shortener/internal/repository/redis/link_cache.go` | new — Redis implementation |
| `services/shortener/internal/service/link_service.go` | extend `Resolve` with cache path; constructor takes cache |
| `services/shortener/internal/service/link_service_test.go` | add mockLinkCache + 3 cache scenarios |
| `services/shortener/internal/app/app.go` | add `initRedis`, register closer, pass redis to container |
| `services/shortener/internal/app/di.go` | add `linkCache` field, wire into service |
| `go.mod` / `go.sum` | add `redis/go-redis/v9` |
| `.env` | add `REDIS_ADDR=localhost:6379` |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add `redis/go-redis/v9` dependency
- [x] add Redis config fields
- [x] create `LinkCache` interface in domain
- [x] implement Redis cache repository
- [x] extend `LinkService.Resolve` with cache path + graceful degradation
- [x] wire Redis client and cache through `App` and `container`
- [x] add service unit tests for hit / miss / cache-error
- [x] run `go test ./...`, `go build ./...`, `go vet ./...`
- [x] pass review or resolve review findings

## Validation

- `go test ./...` — all tests pass
- `go build ./...` — clean
- `go vet ./...` — clean
- manual: `make run`, create link, hit `/{shortCode}` twice, verify second call has no PG
  query in logs (when `max_clicks = 0`)
- manual: `docker compose stop redis`, hit `/{shortCode}` — still returns 302 with warn log

## Review Notes

- implementation updated after review: unlimited-link cache hits now use `TouchRedirect` so
  PostgreSQL remains the authoritative redirect counter without a read-back query
- `go build ./...`, `go vet ./...`, `go test ./...` all pass
- service unit tests cover cache hit + touch, touch failure, miss+warm, cache-error fallback,
  capped-link path, and computeTTL edges

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] Cache-hit path for unlimited links now calls `TouchRedirect` before returning cached data,
  so PostgreSQL remains the authoritative `redirect_count` source while the hot path still
  avoids a read-back query.

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] Task notes clarified that Redis boot connectivity is required, but readiness intentionally
  stays PostgreSQL-only because Redis is a fail-open dependency for redirects.
