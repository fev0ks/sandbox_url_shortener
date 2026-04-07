# E03-003 - Add Create Endpoint Rate Limiting

Status: backlog

Epic: E03 - Redirect Performance And Safety

## Summary

Protect `POST /api/v1/links` from abuse with a per-IP rate limit backed by Redis (token bucket
or fixed window). Other endpoints stay unlimited for now. The limiter must fail-open if Redis
is down so legitimate creates don't break during a Redis outage.

## Scope

- per-client-IP rate limit on `POST /api/v1/links` only
- Redis-backed counter so multiple API replicas share state
- standard `429 Too Many Requests` response with `Retry-After` header on limit exceeded
- fail-open on Redis errors (log warn, allow request)
- excludes per-user limits (no auth yet)
- excludes redirect or read endpoint limits
- excludes WAF / DDoS protection (out of scope for application code)

## Acceptance Criteria

- `POST /api/v1/links` enforces a default of `10 requests / minute / IP`
- on limit exceeded the response is `429` with the standard JSON error shape and a
  `Retry-After` header in seconds
- the limit values (`RATE_LIMIT_CREATE_PER_MIN`) are configurable via env vars with a sensible
  default
- client IP is extracted via `X-Forwarded-For` first segment if present, else `RemoteAddr`
- Redis errors result in fail-open behavior: request is allowed, warning logged
- the limiter is implemented as HTTP middleware applied only to the create route, not the whole
  mux
- unit tests cover: under-limit, at-limit, over-limit, Redis-error fail-open
- `go build`, `go vet`, `go test ./...` all pass

## Assumptions

- E03-001 is complete (Redis client available in container)
- a fixed-window counter is acceptable for MVP; sliding window or token bucket can be a follow-up
- IP-based limiting is acceptable until authenticated users exist; the open question in the
  epic is acknowledged
- writing a hand-rolled INCR-with-EXPIRE limiter is preferred over pulling in
  `go-redis/redis_rate` to keep dependencies small (revisit if it gets complex)

## Implementation Notes

### Algorithm: fixed window with `INCR` + `EXPIRE`

```
key = ratelimit:create:{ip}:{minute_bucket}
count = INCR(key)
if count == 1:
    EXPIRE(key, 60)
if count > limit:
    return 429
```

`minute_bucket` = `time.Now().Unix() / 60`. Each minute gets its own key, naturally garbage
collected by the EXPIRE.

### Middleware

```go
// internal/platform/middleware/ratelimit.go
func RateLimitCreate(cache RateLimiter, limit int, log *slog.Logger) func(http.Handler) http.Handler
```

`RateLimiter` is a small interface (just `Incr` + `Expire`) defined in middleware so the
middleware doesn't depend on the Redis package directly. The Redis cache from E03-001 can
implement it, or a separate small struct can wrap the client.

### Wiring

In `initHTTPServer`, only the create route gets the wrapped handler:

```go
mux.Handle("POST /api/v1/links",
    middleware.RateLimitCreate(a.di.rateLimiter, a.cfg.RateLimitCreatePerMin, a.log)(
        http.HandlerFunc(a.di.linkHandler.Create)))
```

Other routes are unchanged.

### Config

```go
RateLimitCreatePerMin int `env:"RATE_LIMIT_CREATE_PER_MIN,default=10"`
```

### Error response

Reuse the existing `respond.Error` helper with code `rate_limited`, status `429`, and set
`Retry-After: 60` (or seconds remaining in current bucket if computable).

### Files changed

| File | Change |
|------|--------|
| `internal/config/config.go` | add `RateLimitCreatePerMin` |
| `internal/platform/middleware/ratelimit.go` | new — rate-limit middleware + RateLimiter interface |
| `internal/platform/middleware/ratelimit_test.go` | new — under/at/over/fail-open tests |
| `internal/app/di.go` | add `rateLimiter` field if not reusing the cache |
| `internal/app/app.go` | wrap create route with the limiter middleware |
| `.env` | add `RATE_LIMIT_CREATE_PER_MIN=10` |

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] add config field with default
- [ ] design `RateLimiter` interface and Redis implementation
- [ ] implement `RateLimitCreate` middleware with fail-open
- [ ] add unit tests with mock RateLimiter (under/at/over/error)
- [ ] wrap only `POST /api/v1/links` in `initHTTPServer`
- [ ] manual smoke test against running stack
- [ ] run `go test ./...`, `go build ./...`, `go vet ./...`
- [ ] pass review or resolve review findings

## Validation

- `go test ./...` — all pass
- `go build ./...` — clean
- `go vet ./...` — clean
- manual: `for i in {1..15}; do curl -X POST .../api/v1/links -d '{"original_url":"x"}'; done`
  — first 10 return 201, remaining return 429 with Retry-After
- manual: `docker compose stop redis` then create — still returns 201 with warn log

## Review Notes

- pending

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
