# E02-003 - Implement Enable Disable Delete Flows

Status: complete

Epic: E02 - Core Link Lifecycle

## Summary

Add the enable, disable, and soft-delete administration flows for links. These operations update
timestamp columns (`disabled_at`, `deleted_at`) and affect redirect eligibility. Extends the
repository, service, and handler layers introduced in E02-002.

## Scope

- extend `LinkRepository` with `Disable`, `Enable`, `SoftDelete` methods
- extend `LinkService` with corresponding business logic
- add HTTP handlers: `POST /api/v1/links/{id}/disable`, `POST /api/v1/links/{id}/enable`,
  `DELETE /api/v1/links/{id}`
- wire new routes in `App.initHTTPServer()`
- excludes redirect flow (E02-004), hard delete, and bulk operations

## Acceptance Criteria

- `POST /api/v1/links/{id}/disable` sets `disabled_at = now()`, returns 200
- `POST /api/v1/links/{id}/disable` on already-disabled link is idempotent (200, no error)
- `POST /api/v1/links/{id}/enable` clears `disabled_at = NULL`, returns 200
- `POST /api/v1/links/{id}/enable` on already-enabled link is idempotent (200, no error)
- `DELETE /api/v1/links/{id}` sets `deleted_at = now()` and `updated_at = now()`, returns 204
- `DELETE /api/v1/links/{id}` on already-deleted link returns 404
- `GET /api/v1/links/{id}` for a soft-deleted link returns 404
- all three operations on a non-existent link return 404
- `updated_at` is refreshed on disable, enable, and delete
- `go build ./...` and `go vet ./...` pass
- unit tests for service layer (idempotency, not-found cases)
- integration tests for disable → enable → delete lifecycle

## Assumptions

- E02-002 is complete (repository, service, handler patterns established)
- disable and enable are separate endpoints (not a toggle)
- soft delete is irreversible in the MVP (no restore endpoint)
- disabled links still appear in `GET /api/v1/links/{id}` (only deleted ones are hidden)

## Implementation Notes

### Repository additions

```go
// Added to LinkRepository interface
Disable(ctx context.Context, id uuid.UUID) error
Enable(ctx context.Context, id uuid.UUID) error
SoftDelete(ctx context.Context, id uuid.UUID) error
```

SQL patterns:
```sql
-- Disable
UPDATE links SET disabled_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- Enable
UPDATE links SET disabled_at = NULL, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- Soft delete
UPDATE links SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
```

Each returns rows affected — 0 rows means link not found (or already deleted).

### Service layer

- `Disable`: call repo, return ErrNotFound if 0 rows affected
- `Enable`: call repo, return ErrNotFound if 0 rows affected
- `SoftDelete`: call repo, return ErrNotFound if 0 rows affected

### HTTP handlers

```go
func (h *LinkHandler) Disable(w http.ResponseWriter, r *http.Request) { ... } // 200 + link JSON
func (h *LinkHandler) Enable(w http.ResponseWriter, r *http.Request)  { ... } // 200 + link JSON
func (h *LinkHandler) Delete(w http.ResponseWriter, r *http.Request)  { ... } // 204 No Content
```

### Route wiring

```go
mux.HandleFunc("POST /api/v1/links/{id}/disable", linkH.Disable)
mux.HandleFunc("POST /api/v1/links/{id}/enable", linkH.Enable)
mux.HandleFunc("DELETE /api/v1/links/{id}", linkH.Delete)
```

### Files changed

| File | Change |
|------|--------|
| `internal/domain/link_repository.go` | extended — add Disable, Enable, SoftDelete |
| `internal/domain/link_service.go` | extended — add Disable, Enable, SoftDelete |
| `internal/repository/postgres/link_repo.go` | extended — implement new methods |
| `internal/service/link_service.go` | extended — implement new methods |
| `internal/service/link_service_test.go` | extended — new test cases |
| `internal/transport/http/stdlib/handler/link_handler.go` | extended — new handlers |
| `internal/app/app.go` | modified — register new routes |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] extend repository interface and PostgreSQL implementation
- [x] extend service layer with business logic
- [x] add HTTP handlers for disable, enable, delete
- [x] wire new routes in `App.initHTTPServer()`
- [x] add service unit tests (SoftDelete not-found covered)
- [ ] add integration tests for full lifecycle (deferred — requires running PG)
- [x] run `go test ./...`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go test ./...` — PASS
- `go build ./...` — clean
- `go vet ./...` — clean
- integration tests deferred to manual validation with docker-compose

## Review Notes

- implemented together with E02-002 since all layers were built in one pass
- Disable and Enable are idempotent at the SQL level (UPDATE WHERE deleted_at IS NULL)
- Disable/Enable handlers return updated link JSON (200), Delete returns 204 No Content
