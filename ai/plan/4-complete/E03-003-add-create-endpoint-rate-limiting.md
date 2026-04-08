# E03-003 - Add Create Endpoint Rate Limiting

Status: complete

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
- the limiter is implemented as create-route middleware logic applied only to fresh
  `POST /api/v1/links` requests, not the whole mux
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
// internal/platform/httppolicy/ratelimit.go
func RateLimitCreate(cache RateLimiter, limit int, log *slog.Logger) func(http.Handler) http.Handler
```

`RateLimiter` is a small interface defined in route-policy code so the HTTP layer doesn't
depend on the Redis package directly. The limiter instance itself stays generic; route-level
policy supplies a key builder (for example client IP under the `ratelimit:create` namespace)
before calling `Allow(...)`.

### Wiring

Routing now lives in `services/shortener/internal/app/router.go` and feature handlers
self-register via `link.Handler.Register(mux)`. Two viable wiring options:

**Option A — per-route limiter inside the link package** (preferred): inject a
concrete create-rate-limit dependency into `link.Handler`, and apply it only to
the fresh create execution path after idempotency replay/conflict checks:

```go
// link/handler.go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
    if replayedFromIdempotencyCache(r) {
        // return cached 201 / 409 without spending abuse budget
        return
    }
    h.serveFreshCreate(w, r) // applies limiter only here
}
```

**Option B — wire in router.go** by registering the create route manually before
calling `Register` for the rest. Less clean because it splits one feature's routes
across two files; pick A.

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
| `services/shortener/internal/config/config.go` | add `RateLimitCreatePerMin` |
| `internal/platform/httppolicy/ratelimit.go` | new — rate-limit route policy + Limiter interface |
| `internal/platform/httppolicy/ratelimit_test.go` | new — under/at/over/fail-open tests |
| `services/shortener/internal/app/di.go` | add `rateLimiter` field if not reusing the cache |
| `services/shortener/internal/transport/http/stdlib/link/handler.go` | accept optional limiter middleware, wrap only the create route in `Register` |
| `services/shortener/internal/app/di.go` | construct `link.NewHandler` with the limiter middleware |
| `.env` | add `RATE_LIMIT_CREATE_PER_MIN=10` |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add config field with default
- [x] design `RateLimiter` interface and Redis implementation
- [x] implement `RateLimitCreate` middleware with fail-open
- [x] add unit tests with mock RateLimiter (under/at/over/error)
- [x] wrap only `POST /api/v1/links` inside `link.Handler.Register`
- [x] manual smoke test against running stack
- [x] run `go test ./...`, `go build ./...`, `go vet ./...`
- [x] pass review or resolve review findings

## Validation

- `go test ./...` — all pass
- `go build ./...` — clean
- `go vet ./...` — clean
- manual: `for i in {1..15}; do curl -X POST .../api/v1/links -d '{"original_url":"x"}'; done`
  — first 10 return 201, remaining return 429 with Retry-After
- manual: `docker compose stop redis` then create — still returns 201 with warn log

## Review Notes

- implementation updated after review: idempotent replays now bypass rate-limit enforcement,
  while fresh creates still go through the same limiter wired through `link.Handler`
- `go build ./...`, `go vet ./...`, `go test ./...` all pass
- limiter stays as reusable platform HTTP policy; `link.Handler` owns the create-route assembly
  and applies the injected create-rate-limit only to fresh create attempts. The Redis limiter
  instance is now generic and route policy supplies a client-IP key builder under the
  `ratelimit:create` namespace. Backend lives in `repository/redis/fixed_window_limiter.go`
- ratelimit_test.go covers under-limit, over-limit (with Retry-After), fail-open, and
  X-Forwarded-For/RemoteAddr extraction; handler tests cover replay bypassing the limiter

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] Idempotency replay/conflict handling now runs before the fresh-create rate-limit path, so
  duplicate requests with the same `Idempotency-Key` replay cached responses without spending
  abuse budget or being throttled with `429`.

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
