# E03-002 - Implement Cache Invalidation For Link State Changes

Status: backlog

Epic: E03 - Redirect Performance And Safety

## Summary

Invalidate the Redis cache entry for a link whenever its state changes through disable, enable,
delete, or future update flows. Cache invalidation is the responsibility of the service layer,
not the handler. Failures must not block the underlying state change.

## Scope

- invalidate `link:code:{short_code}` on `LinkService.Disable`
- invalidate on `LinkService.Enable`
- invalidate on `LinkService.SoftDelete`
- service layer ignores cache invalidation errors (log only)
- excludes content updates (no update endpoint exists yet)
- excludes proactive warm-up after enable

## Acceptance Criteria

- after `Disable`, the next redirect for that short code consults PostgreSQL (cache miss)
- after `Enable`, the next redirect re-warms the cache from PostgreSQL
- after `SoftDelete`, the cache entry is gone and the redirect returns 404
- if Redis is down, disable/enable/delete still succeed and return 200/204
- service tests cover all three flows with a mock cache and verify `Delete` is called
- service tests cover the "Redis returns error during invalidation" path and verify the state
  change still succeeds
- `go build`, `go vet`, `go test ./...` all pass

## Assumptions

- E03-001 is complete (`LinkCache` interface and Redis implementation exist)
- the service already has `cache` injected; no new wiring needed
- short_code is required for invalidation; service must fetch the link first if it only has the
  ID, OR the existing repo methods can return the short_code as part of the update
- preferred approach: fetch the link before mutating, so we know the short_code without an extra
  round trip after; alternative: change the repo signatures to `RETURNING short_code`

## Implementation Notes

- chosen approach: use `RETURNING short_code` in the UPDATE statements so the repo returns the
  affected short_code without a separate `GetByID` call

  ```sql
  UPDATE links SET disabled_at = now(), updated_at = now()
  WHERE id = $1 AND deleted_at IS NULL
  RETURNING short_code;
  ```

- repo signatures change from `Disable(ctx, id) error` to `Disable(ctx, id) (string, error)`;
  same for `Enable` and `SoftDelete`. `ErrNotFound` returned when no row returned.
- service method shape:

  ```go
  func (s *LinkService) Disable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
      shortCode, err := s.repo.Disable(ctx, id)
      if err != nil { return nil, err }
      if cacheErr := s.cache.Delete(ctx, shortCode); cacheErr != nil {
          s.log.WarnContext(ctx, "cache invalidation failed",
              "short_code", shortCode, "op", "disable", "error", cacheErr)
      }
      return s.repo.GetByID(ctx, id)
  }
  ```

- handler layer is unchanged
- the `cache` field on `LinkService` is the same `domain.LinkCache` interface from E03-001

### Files changed

| File | Change |
|------|--------|
| `internal/domain/link_repository.go` | change Disable/Enable/SoftDelete return types to `(string, error)` |
| `internal/repository/postgres/link_repo.go` | use `RETURNING short_code` in UPDATEs |
| `internal/service/link_service.go` | call `cache.Delete` after each mutation, swallow errors |
| `internal/service/link_service_test.go` | extend mocks; add 6 new test cases (3 happy + 3 cache-error) |

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] update `LinkRepository` interface signatures
- [ ] update PostgreSQL repo with `RETURNING short_code`
- [ ] update mock repo in service tests
- [ ] add cache invalidation to `Disable`, `Enable`, `SoftDelete`
- [ ] add tests for happy path invalidation
- [ ] add tests for cache-error tolerance
- [ ] run `go test ./...`, `go build ./...`, `go vet ./...`
- [ ] pass review or resolve review findings

## Validation

- `go test ./...` — all pass
- `go build ./...` — clean
- `go vet ./...` — clean
- manual: create link → redirect (warm cache) → disable → redirect → 404 (PG path, cache empty)
- manual: create link → redirect → `docker compose stop redis` → disable → 200 with warn log

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
