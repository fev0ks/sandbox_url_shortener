# E02-001 - Design Link Model And PostgreSQL Migrations

Status: complete

Epic: E02 - Core Link Lifecycle

## Summary

Define the `Link` domain model in Go, create the initial PostgreSQL migration for the `links` table
with indexes, and implement short code generation. This task establishes the data foundation that all
other E02 tasks build on.

## Scope

- create `internal/domain/link.go` with the `Link` struct (13 fields)
- create migration `000001_create_links_table.up.sql` and `.down.sql` using `golang-migrate`
- implement short code generator in `internal/shortcode/` using `crypto/rand` + base58 alphabet
- add `internal/domain/errors.go` with domain error sentinels (ErrNotFound, ErrShortCodeCollision)
- excludes HTTP handlers, service layer, and repository implementation

## Acceptance Criteria

- `Link` struct has all 13 fields matching the PostgreSQL schema in `data_and_logging_strategy.md`
- `id` uses UUIDv7 (`google/uuid` v7 generation)
- migration up creates `links` table with correct column types, defaults, and constraints
- migration down drops the `links` table
- unique partial index: `UNIQUE (short_code) WHERE deleted_at IS NULL`
- index on `(created_at DESC)`
- short code generator produces 7-character base58 strings using `crypto/rand`
- short code generator validates output against the explicit base58 alphabet
- `go build ./...` and `go vet ./...` pass
- unit tests for short code generation cover: correct length, valid alphabet, uniqueness over 1000 runs

## Assumptions

- `golang-migrate` is already a dependency (added in E01)
- migrations live in `migrations/postgres/`
- `google/uuid` is added for UUIDv7 generation
- the `Link` struct uses `time.Time` for timestamp fields and `*time.Time` for nullable ones
- `created_by` is `*uuid.UUID` (nullable until user identity exists)

## Implementation Notes

### Link domain model

```go
// internal/domain/link.go
type Link struct {
    ID             uuid.UUID
    ShortCode      string
    OriginalURL    string
    Title          *string
    CreatedBy      *uuid.UUID
    CreatedAt      time.Time
    UpdatedAt      time.Time
    ExpiresAt      *time.Time
    MaxClicks      int64
    RedirectCount  int64
    DisabledAt     *time.Time
    DeletedAt      *time.Time
    LastAccessedAt *time.Time
}
```

### PostgreSQL migration

```sql
-- 000001_create_links_table.up.sql
CREATE TABLE links (
    id              UUID PRIMARY KEY,
    short_code      TEXT NOT NULL,
    original_url    TEXT NOT NULL,
    title           TEXT,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ,
    max_clicks      BIGINT NOT NULL DEFAULT 0,
    redirect_count  BIGINT NOT NULL DEFAULT 0,
    disabled_at     TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ,
    last_accessed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_links_short_code_active
    ON links (short_code) WHERE deleted_at IS NULL;

CREATE INDEX idx_links_created_at
    ON links (created_at DESC);
```

### Short code generator

- alphabet: `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz` (58 chars)
- length: 7
- uses `crypto/rand.Read` to fill a byte buffer, maps each byte modulo 58 to alphabet
- exported `Generate() (string, error)` function
- constant `Length = 7` and `Alphabet` exported for validation

### Domain errors

```go
// internal/domain/errors.go
var (
    ErrNotFound           = errors.New("link not found")
    ErrShortCodeCollision = errors.New("short code collision")
)
```

### Files changed

| File | Change |
|------|--------|
| `internal/domain/link.go` | new — Link struct |
| `internal/domain/errors.go` | new — domain error sentinels |
| `internal/shortcode/shortcode.go` | new — base58 short code generator |
| `internal/shortcode/shortcode_test.go` | new — generator tests |
| `migrations/postgres/000001_create_links_table.up.sql` | new — create table + indexes |
| `migrations/postgres/000001_create_links_table.down.sql` | new — drop table |
| `go.mod` / `go.sum` | add `google/uuid` |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add `google/uuid` dependency
- [x] create `internal/domain/link.go`
- [x] create `internal/domain/errors.go`
- [x] create `internal/shortcode/shortcode.go`
- [x] create `internal/shortcode/shortcode_test.go`
- [x] create migration up and down files
- [x] run `go test ./...`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go test ./internal/shortcode/...` — PASS (0.395s)
- `go build ./...` — clean
- `go vet ./...` — clean

## Review Notes

- all 13 Link fields match data_and_logging_strategy.md schema
- ErrLinkGone added proactively for E02-004
- google/uuid v1.6.0 added
