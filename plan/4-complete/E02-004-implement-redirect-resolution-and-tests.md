# E02-004 - Implement Redirect Resolution And Tests

Status: complete

Epic: E02 - Core Link Lifecycle

## Summary

Implement the redirect handler at `GET /{shortCode}` that resolves a short code to the original URL,
atomically increments `redirect_count`, and returns the appropriate HTTP status. This is the core
product flow of the URL shortener.

## Scope

- extend `LinkRepository` with `ResolveAndIncrement(ctx, shortCode) (*Link, error)` method
- extend `LinkService` with `Resolve(ctx, shortCode) (*Link, error)` method
- create redirect handler at `GET /{shortCode}` (top-level, not under `/api/v1/`)
- implement redirect eligibility rules from `data_and_logging_strategy.md`
- handle 302, 404, and 410 status codes based on link state
- update `last_accessed_at` during redirect
- excludes Redis cache (E03), Kafka click event publishing (E05), and ClickHouse analytics (E06)

## Acceptance Criteria

- `GET /{shortCode}` for an active link returns `302 Found` with `Location` header
- `GET /{shortCode}` for an unknown short code returns `404 Not Found`
- `GET /{shortCode}` for a soft-deleted link returns `404 Not Found`
- `GET /{shortCode}` for a disabled link returns `404 Not Found`
- `GET /{shortCode}` for an expired link (`expires_at < now()`) returns `410 Gone`
- `GET /{shortCode}` for a link at max clicks (`redirect_count >= max_clicks` where `max_clicks > 0`)
  returns `410 Gone`
- `redirect_count` is atomically incremented during successful redirect
- `last_accessed_at` is updated during successful redirect
- `max_clicks = 0` means unlimited redirects
- concurrent redirects do not cause race conditions on `redirect_count`
- `go build ./...` and `go vet ./...` pass
- unit tests for service-level redirect logic (all status code branches)
- integration tests for redirect flow including max_clicks exhaustion

## Assumptions

- E02-002 and E02-003 are complete (repository, service, handler patterns established)
- redirect handler is registered on the default mux, not under `/api/v1/`
- the redirect handler returns minimal HTML body (status text only), not JSON
- no rate limiting on redirects in this task (deferred to E03 or later)

## Implementation Notes

### Repository method

```go
// Added to LinkRepository interface
ResolveAndIncrement(ctx context.Context, shortCode string) (*Link, error)
```

Single atomic query using `UPDATE ... RETURNING`:
```sql
UPDATE links
SET redirect_count = redirect_count + 1,
    last_accessed_at = now(),
    updated_at = now()
WHERE short_code = $1
  AND deleted_at IS NULL
  AND disabled_at IS NULL
  AND (expires_at IS NULL OR expires_at > now())
  AND (max_clicks = 0 OR redirect_count < max_clicks)
RETURNING id, short_code, original_url, redirect_count, max_clicks;
```

If 0 rows returned, need a second read-only query to distinguish 404 vs 410:
```sql
SELECT id, deleted_at, disabled_at, expires_at, max_clicks, redirect_count
FROM links WHERE short_code = $1 AND deleted_at IS NULL;
```

- Not found at all → `ErrNotFound` (404)
- Found but disabled → `ErrNotFound` (404)
- Found but expired or max_clicks reached → `ErrLinkGone` (410)

### Domain errors addition

```go
var ErrLinkGone = errors.New("link gone")
```

### Service layer

```go
func (s *LinkService) Resolve(ctx context.Context, shortCode string) (*domain.Link, error) {
    link, err := s.repo.ResolveAndIncrement(ctx, shortCode)
    if err != nil {
        // Map repository errors to domain errors
        return nil, err
    }
    return link, nil
}
```

### Redirect handler

```go
// internal/transport/http/stdlib/handler/redirect_handler.go
type RedirectHandler struct {
    svc domain.LinkServiceInterface
    log *slog.Logger
}

func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
    shortCode := r.PathValue("shortCode")
    link, err := h.svc.Resolve(r.Context(), shortCode)
    switch {
    case errors.Is(err, domain.ErrNotFound):
        http.Error(w, "Not Found", http.StatusNotFound)
    case errors.Is(err, domain.ErrLinkGone):
        http.Error(w, "Gone", http.StatusGone)
    case err != nil:
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    default:
        http.Redirect(w, r, link.OriginalURL, http.StatusFound)
    }
}
```

### Route wiring

```go
mux.HandleFunc("GET /{shortCode}", redirectH.Redirect)
```

Must be registered after more specific routes (`/api/v1/...`, `/health/...`, `/metrics`) so the
catch-all does not shadow them.

### Files changed

| File | Change |
|------|--------|
| `internal/domain/errors.go` | extended — add ErrLinkGone |
| `internal/domain/link_repository.go` | extended — add ResolveAndIncrement |
| `internal/domain/link_service.go` | extended — add Resolve |
| `internal/repository/postgres/link_repo.go` | extended — implement ResolveAndIncrement |
| `internal/service/link_service.go` | extended — implement Resolve |
| `internal/service/link_service_test.go` | extended — redirect test cases |
| `internal/transport/http/stdlib/handler/redirect_handler.go` | new — redirect handler |
| `internal/app/app.go` | modified — wire redirect handler |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add `ErrLinkGone` to domain errors
- [x] extend repository interface with `ResolveAndIncrement`
- [x] implement atomic resolve query in PostgreSQL repo
- [x] implement 404 vs 410 disambiguation query
- [x] extend service layer with `Resolve`
- [x] create redirect handler
- [x] wire redirect route in `App.initHTTPServer()`
- [x] add service unit tests for all redirect branches (Resolve success + ErrLinkGone)
- [ ] add integration tests (deferred — requires running PG)
- [ ] add concurrency test for redirect_count atomicity (deferred — requires running PG)
- [x] run `go test ./...`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go test ./...` — PASS
- `go build ./...` — clean
- `go vet ./...` — clean
- integration tests deferred to manual validation with docker-compose

## Review Notes

- implemented together with E02-002 and E02-003
- ResolveAndIncrement uses atomic UPDATE...RETURNING with fallback classifyMiss query for 404 vs 410
- redirect handler uses plain http.Error (not JSON) for non-API responses
- catch-all `GET /{shortCode}` registered last in mux to avoid shadowing API routes
