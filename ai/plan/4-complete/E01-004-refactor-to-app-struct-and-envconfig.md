# E01-004 - Refactor To App Struct And go-envconfig

Status: complete

Epic: E01 - Platform Bootstrap

## Summary

Refactor `cmd/api-http-stdlib` and `cmd/worker` to use an `App` composition root in `internal/app/`
with a structured init chain, LIFO closers, and `sethvargo/go-envconfig` for configuration. This
aligns the project with the DI patterns documented in `docs/di_reference_skibookers.md`.

## Scope

- replace `internal/config/` manual env loading with `go-envconfig` struct tags
- create `internal/app/app.go` with `App` struct as composition root for `api-http-stdlib`
- create `internal/app/worker_app.go` with `WorkerApp` struct for `worker`
- move server setup, DB pool init, middleware wiring, and graceful shutdown from `main.go` into
  the `App`/`WorkerApp` init chain
- implement LIFO closers pattern for resource cleanup
- keep `cmd/api-http-stdlib/main.go` and `cmd/worker/main.go` thin (just `NewApp` + `Run`)
- update tests for new config loading
- excludes business logic, new endpoints, or infrastructure changes

## Acceptance Criteria

- `cmd/api-http-stdlib/main.go` is under 20 lines (NewApp + Run + error handling)
- `cmd/worker/main.go` is under 20 lines
- `App` struct owns the full init chain: config → logger → DB pool → HTTP server
- config uses `go-envconfig` struct tags with `required`, `default` directives
- missing required env var produces a clear error from `go-envconfig`
- LIFO closers: resources close in reverse init order on shutdown
- all existing tests pass
- `go build ./...` and `go vet ./...` pass

## Assumptions

- `go-envconfig` is added as a dependency (`sethvargo/go-envconfig`)
- the `.env` file format stays the same; only Go-side loading changes
- `WorkerApp` does not connect to PostgreSQL (same as current behavior)
- middleware wiring and health/metrics handlers stay unchanged in behavior

## Implementation Notes

### Config refactor

Replace `internal/config/config.go`:

```go
type Config struct {
    AppPort         string `env:"APP_PORT,default=8080"`
    WorkerAdminPort string `env:"WORKER_ADMIN_PORT,default=9090"`
    LogLevel        string `env:"LOG_LEVEL,default=INFO"`
    Env             string `env:"APP_ENV,default=local"`
    DBDSN           string `env:"DB_DSN,required"`
    DBMaxConns      int32  `env:"DB_MAX_CONNS,default=10"`
}
```

Add a `Validate()` method for cross-field rules (e.g., `LogLevel` must be one of
DEBUG/INFO/WARN/ERROR). `go-envconfig` handles required/default/type; `Validate()` handles
semantic rules.

### App struct

```go
// internal/app/app.go
type App struct {
    cfg     *config.Config
    log     *slog.Logger
    pool    *pgxpool.Pool
    srv     *http.Server
    closers []func() error
}

func NewApp(ctx context.Context) (*App, error) {
    a := &App{}
    if err := a.initConfig(ctx); err != nil { return nil, err }
    a.initLogger()
    if err := a.initDB(ctx); err != nil { return nil, err }
    a.initHTTPServer()
    return a, nil
}

func (a *App) Run(ctx context.Context) error { ... }
func (a *App) Shutdown(ctx context.Context) { ... }
```

### WorkerApp struct

```go
// internal/app/worker_app.go
type WorkerApp struct {
    cfg     *config.Config
    log     *slog.Logger
    srv     *http.Server
    closers []func() error
}
```

Same pattern, no DB pool.

### Thin main

```go
func main() {
    ctx := context.Background()
    a, err := app.NewApp(ctx)
    if err != nil {
        log.Fatalf("failed to init app: %v", err)
    }
    a.Run(ctx)
}
```

### Closers

```go
func (a *App) addCloser(fn func() error) {
    a.closers = append(a.closers, fn)
}

// In initDB:
a.addCloser(func() error { pool.Close(); return nil })

// In Shutdown:
for i := len(a.closers) - 1; i >= 0; i-- {
    _ = a.closers[i]()
}
```

### Files changed

| File | Change |
|------|--------|
| `internal/config/config.go` | rewrite with `go-envconfig` tags + `Validate()` |
| `internal/config/config_test.go` | update tests for new loading |
| `internal/app/app.go` | new — App struct for api-http-stdlib |
| `internal/app/worker_app.go` | new — WorkerApp struct for worker |
| `cmd/api-http-stdlib/main.go` | slim down to NewApp + Run |
| `cmd/worker/main.go` | slim down to NewWorkerApp + Run |
| `go.mod` / `go.sum` | add `sethvargo/go-envconfig` |

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add `sethvargo/go-envconfig` dependency
- [x] rewrite `internal/config/config.go` with struct tags + `Validate()`
- [x] update `internal/config/config_test.go`
- [x] create `internal/app/app.go` with init chain and LIFO closers
- [x] create `internal/app/worker_app.go`
- [x] slim `cmd/api-http-stdlib/main.go`
- [x] slim `cmd/worker/main.go`
- [x] run `go test ./...`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go test ./...` — all tests pass
- `go build ./...` — compiles
- `go vet ./...` — clean
- unset `DB_DSN`, start binary — clear `go-envconfig` error message
- set `LOG_LEVEL=TRACE`, start binary — `Validate()` rejects it
- `make run` — Docker Compose stack starts and becomes healthy

## Review Notes

- complete
