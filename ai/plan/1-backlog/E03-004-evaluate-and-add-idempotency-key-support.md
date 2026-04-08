# E03-004 - Evaluate And Add Idempotency Key Support

Status: backlog

Epic: E03 - Redirect Performance And Safety

## Summary

Add optional `Idempotency-Key` header support to `POST /api/v1/links` so clients can retry safely
after network failures without creating duplicate links. The cached response is keyed by the
header value plus the request body hash. This is the standard Stripe-style pattern.

## Scope

- accept optional `Idempotency-Key` header on `POST /api/v1/links`
- Redis-backed cache of response for `(key, body_hash)` pairs
- replay cached response on duplicate request within TTL
- reject conflicting requests (same key, different body) with `409 Conflict`
- fail-open on Redis errors (log warn, treat request as fresh)
- excludes other endpoints
- excludes idempotency for delete/disable/enable (those are already idempotent or out of scope)

## Acceptance Criteria

- `POST /api/v1/links` without `Idempotency-Key` header behaves exactly as today
- `POST /api/v1/links` with `Idempotency-Key` header and a fresh key creates the link normally
  AND caches the full response in Redis under `idempotency:create:{key}` with 24h TTL
- `POST /api/v1/links` with the same key + same body within TTL replays the cached response
  (same status code, same body, same `Location` if any)
- `POST /api/v1/links` with the same key + different body returns `409 Conflict` with the
  standard error JSON
- key length is bounded (max 255 chars), invalid keys return `400`
- if Redis is down, the request is processed normally and a warn is logged (fail-open)
- unit tests cover: no header, fresh key, replay, conflict, Redis-error fail-open
- `go build`, `go vet`, `go test ./...` all pass

## Assumptions

- E03-001 is complete (Redis client available)
- the cached response is small (a single LinkResponse JSON), so storing the full body in Redis
  is acceptable
- body hash is `sha256(raw_body_bytes)` truncated to hex; sufficient to detect tampering
- 24h TTL is enough for retry windows; configurable via env if needed later
- this is a "best-effort idempotency" — under high concurrency two simultaneous requests with
  the same fresh key may both proceed and one will lose the race; this is acceptable for MVP
  (full strict idempotency requires distributed locking and is much heavier)

## Implementation Notes

### Stored value shape

```go
type idempotencyRecord struct {
    BodyHash    string          `json:"body_hash"`
    StatusCode  int             `json:"status_code"`
    ResponseBody json.RawMessage `json:"response_body"`
}
```

Stored as a single JSON blob in Redis under `idempotency:create:{key}`.

### Middleware vs handler

This belongs in the handler (or a thin wrapper) rather than as transport middleware, because
the middleware would need to buffer the full response body to cache it. A wrapper helper in
the handler package keeps it simple:

```go
// services/shortener/internal/transport/http/stdlib/handler/idempotency.go
type IdempotencyStore interface {
    Get(ctx context.Context, key string) (*Record, error)
    Set(ctx context.Context, key string, rec *Record, ttl time.Duration) error
}
```

The `LinkHandler.Create` method:
1. Read header. If empty → existing flow.
2. Read full body. Compute `sha256(body)`.
3. Look up `Get(key)`.
   - If found and `body_hash == computed` → write cached status + body, return.
   - If found and `body_hash != computed` → return 409.
   - If not found → process normally, then `Set(key, record)`.
4. Redis errors at any step → log warn, fall through to normal flow.

### Request/response capture

Use a `httptest.ResponseRecorder` (or a custom buffering writer) inside the handler to capture
the response, then either replay from cache or write through. This buffering is fine because
the create response is small and bounded.

### Files changed

| File | Change |
|------|--------|
| `services/shortener/internal/transport/http/stdlib/handler/idempotency.go` | new — IdempotencyStore interface + helper |
| `internal/repository/redis/idempotency_store.go` | new — Redis implementation |
| `services/shortener/internal/transport/http/stdlib/handler/link_handler.go` | extend `Create` with idempotency wrap |
| `services/shortener/internal/transport/http/stdlib/handler/link_handler_test.go` | new — handler-level tests with mock store |
| `services/shortener/internal/app/di.go` | add `idempotencyStore` field, pass to handler |
| `services/shortener/internal/config/config.go` | add `IdempotencyTTL` (default `24h`) |

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] design `IdempotencyStore` interface
- [ ] implement Redis store
- [ ] add idempotency wrap to `LinkHandler.Create`
- [ ] add handler tests with mock store (5 scenarios)
- [ ] wire store through `container`
- [ ] manual smoke test
- [ ] run `go test ./...`, `go build ./...`, `go vet ./...`
- [ ] pass review or resolve review findings

## Validation

- `go test ./...` — all pass
- `go build ./...` — clean
- `go vet ./...` — clean
- manual:
  - `curl -H "Idempotency-Key: abc" ... -d '{"original_url":"https://a.com"}'` → 201
  - same command again → 201 with same response body
  - same key, different `original_url` → 409
  - delete the key from Redis, retry → 201 with new ID
- manual: `docker compose stop redis` then create with key → still 201 with warn log

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
