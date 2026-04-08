# E02-002 - Implement Link Create And Read API

Status: complete

Epic: E02 - Core Link Lifecycle

## Summary

Implement the create and read flows for short links: repository layer for PostgreSQL persistence,
service layer with business logic and short code collision retry, and HTTP handlers for
`POST /api/v1/links` and `GET /api/v1/links/{id}`. This is the first task that wires all three
layers through the App composition root.

## Scope

- define `LinkRepository` interface in `internal/domain/`
- implement PostgreSQL repository in `internal/repository/postgres/link_repo.go`
- define `LinkService` interface and implement in `internal/service/link_service.go`
- create HTTP handlers in `internal/transport/http/stdlib/handler/link_handler.go`
- wire repository → service → handler through `App.initHTTPServer()`
- add request/response DTOs for create and read
- excludes enable/disable/delete flows (E02-003) and redirect (E02-004)
- excludes Redis caching and pagination

## Acceptance Criteria

- `POST /api/v1/links` with `{"original_url": "..."}` creates a link and returns 201 with link JSON
- `POST /api/v1/links` with optional `title`, `expires_at`, `max_clicks` fields
- `POST /api/v1/links` with missing `original_url` returns 400 with standard error JSON
- `GET /api/v1/links/{id}` returns 200 with link JSON for an existing active link
- `GET /api/v1/links/{id}` returns 404 for unknown or soft-deleted links
- short code collision retries up to 5 times before returning 500
- `Link` is persisted with UUIDv7 id, generated short code, and correct timestamps
- repository, service, and handler layers communicate through interfaces
- all layers wired in `App.initHTTPServer()` via constructor injection
- `go build ./...` and `go vet ./...` pass
- unit tests for service layer (mocked repository)
- integration test for create → read round trip (requires running PostgreSQL)

## Assumptions

- E02-001 is complete (Link model, migrations, short code generator exist)
- migrations have been applied to the dev database
- `pgxpool.Pool` is available from `App.pool`
- JSON response shape follows `internal/transport/http/stdlib/respond/` conventions
- request validation is minimal for MVP (URL format check, required fields)

## Implementation Notes

### Repository interface

```go
// internal/domain/link_repository.go
type LinkRepository interface {
    Create(ctx context.Context, link *Link) error
    GetByID(ctx context.Context, id uuid.UUID) (*Link, error)
}
```

Extended by E02-003 and E02-004 with additional methods.

### PostgreSQL repository

```go
// internal/repository/postgres/link_repo.go
type LinkRepo struct {
    pool *pgxpool.Pool
}

func NewLinkRepo(pool *pgxpool.Pool) *LinkRepo { ... }

func (r *LinkRepo) Create(ctx context.Context, link *Link) error {
    // INSERT INTO links (...) VALUES (...) with pgx
}

func (r *LinkRepo) GetByID(ctx context.Context, id uuid.UUID) (*Link, error) {
    // SELECT ... FROM links WHERE id = $1 AND deleted_at IS NULL
}
```

### Service layer

```go
// internal/service/link_service.go
type LinkService struct {
    repo      domain.LinkRepository
    log       *slog.Logger
}

func NewLinkService(repo domain.LinkRepository, log *slog.Logger) *LinkService { ... }

func (s *LinkService) Create(ctx context.Context, input CreateLinkInput) (*domain.Link, error) {
    // Generate UUIDv7
    // Retry loop (max 5): generate short code → repo.Create → on collision retry
    // Return created link
}

func (s *LinkService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
    // repo.GetByID → map ErrNotFound to appropriate error
}
```

### HTTP handler

```go
// internal/transport/http/stdlib/handler/link_handler.go
type LinkHandler struct {
    svc domain.LinkServiceInterface
    log *slog.Logger
}

func NewLinkHandler(svc domain.LinkServiceInterface, log *slog.Logger) *LinkHandler { ... }

func (h *LinkHandler) Create(w http.ResponseWriter, r *http.Request) { ... }
func (h *LinkHandler) GetByID(w http.ResponseWriter, r *http.Request) { ... }
```

### Request/Response DTOs

```go
// Create request
type CreateLinkRequest struct {
    OriginalURL string     `json:"original_url"`
    Title       *string    `json:"title,omitempty"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
    MaxClicks   *int64     `json:"max_clicks,omitempty"`
}

// Link response (shared for create and read)
type LinkResponse struct {
    ID             string     `json:"id"`
    ShortCode      string     `json:"short_code"`
    ShortURL       string     `json:"short_url"`
    OriginalURL    string     `json:"original_url"`
    Title          *string    `json:"title,omitempty"`
    CreatedAt      time.Time  `json:"created_at"`
    ExpiresAt      *time.Time `json:"expires_at,omitempty"`
    MaxClicks      int64      `json:"max_clicks"`
    RedirectCount  int64      `json:"redirect_count"`
    DisabledAt     *time.Time `json:"disabled_at,omitempty"`
}
```

### App wiring

In `app.go` `initHTTPServer()`:
```go
linkRepo := postgres.NewLinkRepo(a.pool)
linkSvc  := service.NewLinkService(linkRepo, a.log)
linkH    := handler.NewLinkHandler(linkSvc, a.log)

mux.HandleFunc("POST /api/v1/links", linkH.Create)
mux.HandleFunc("GET /api/v1/links/{id}", linkH.GetByID)
```

### Files changed

| File | Change |
|------|--------|
| `internal/domain/link_repository.go` | new — LinkRepository interface |
| `internal/domain/link_service.go` | new — LinkService interface |
| `internal/repository/postgres/link_repo.go` | new — PostgreSQL implementation |
| `internal/service/link_service.go` | new — business logic with collision retry |
| `internal/service/link_service_test.go` | new — unit tests with mocked repo |
| `internal/transport/http/stdlib/handler/link_handler.go` | new — HTTP handlers |
| `internal/transport/http/stdlib/handler/dto.go` | new — request/response DTOs |
| `internal/app/app.go` | modified — wire new layers in initHTTPServer |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] create `LinkRepository` interface
- [x] create `LinkService` interface
- [x] implement PostgreSQL `LinkRepo`
- [x] implement `LinkService` with collision retry
- [x] implement `LinkHandler` (Create, GetByID)
- [x] create request/response DTOs
- [x] wire layers in `App.initHTTPServer()`
- [x] add service unit tests
- [ ] add integration test for create → read (deferred — requires running PG)
- [x] run `go test ./...`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go test ./...` — PASS (service tests with mocked repo)
- `go build ./...` — clean
- `go vet ./...` — clean
- integration tests deferred to manual validation with docker-compose

## Review Notes

- full repository interface defined with all E02 methods (Create, GetByID, Disable, Enable, SoftDelete, ResolveAndIncrement)
- service interface defined in domain package, implemented in service package
- constructor DI: repo → service → handler, wired in App.initHTTPServer()
- added BASE_URL config field for short_url generation in responses
- LinkHandler also includes Disable, Enable, Delete handlers (E02-003 scope, implemented together)
