# DI Reference: Skibookers platform-core

Reference analysis of the dependency injection approach used in the skibookers platform-core
service. This document captures the patterns for potential reuse in the URL shortener project.

## Overview

The skibookers platform-core uses **manual constructor-based DI** with no DI framework or container
library. Dependencies flow through explicit constructor parameters, and the `App` struct
orchestrates the entire initialization chain.

## Initialization Sequence

```
main()
  └─ app.NewApp(ctx, log)
       ├─ initConfig()           — env vars via sethvargo/go-envconfig
       ├─ initGracefulShutdown() — SIGINT/SIGTERM channel
       ├─ initModules()          — module container (DB, repos, services)
       ├─ initGrpcServer()       — gRPC server, external clients, interceptors
       └─ initHttpServer()       — HTTP gateway over gRPC
```

Each `init*` method mutates the `App` struct, building up dependencies step by step. Error at any
step aborts startup with a wrapped error.

## App Struct as Composition Root

```go
type App struct {
    log       *logger.ServerLogger
    appConfig config.AppConfig

    http       *http.Server
    grpcServer *AppGrpcServer
    httpServer *AppHttpServer

    closers []func() error         // LIFO resource cleanup
    modules *modules.Container     // business logic modules
    clients *intClients.ClientContainer  // external gRPC clients
    closeCh chan os.Signal

    redisClient *redis.Client
}
```

The `App` struct is the single composition root. It holds all infrastructure references and passes
them down to servers and handlers.

## Module Container

The `modules.Container` groups independent business modules:

```go
type Container struct {
    UserModule  *user_module.Module
    AuthModule  *auth_module.Module
    QuizModule  *quiz_module.Module
    ToolModule  *tool_module.Module
    JobModule   *job_module.Module
}
```

Each module is self-contained and initializes its own:
- config (from environment)
- database connection (gorm)
- migrations
- repository layer
- service layer

Construction follows a fixed pattern inside each module:

```go
func NewModule(ctx, log) (*Module, []func() error, error) {
    m := &Module{}
    m.initConfig(ctx)
    m.initDB(ctx)            // → appends closer
    m.applyDbMigration()
    // constructor chain
    rep := repository.NewRepositoryService(log, db)
    m.UserService = services.NewUserService(log, rep)
    return m, closers, nil
}
```

Modules return their closers to the parent container.

## Dependency Wiring Inside a Module

Example from AuthModule `dependencyInjection()`:

```
Redis client (from config)
  ↓
Messaging client (producer/consumer)
  ↓
Repository service (wraps gorm.DB)
  ↓
BlockService (rate limiting, depends on Redis)
  ↓
OtpService (depends on BlockService, Redis, Messaging)
  ↓
JwtService (depends on config, repository)
  ↓
AuthLinkService (depends on config, Redis, Messaging, Repository, BlockService, JwtService)
  ↓
ClaimsService (JWT extraction)
  ↓
VerifyTokenService (depends on multiple services above)
```

Each constructor takes explicit parameters — no reflection, no tags, no container lookups.

## BaseServer Pattern (Handler DI)

All gRPC handler servers embed a shared `BaseServer`:

```go
type BaseServer struct {
    Log     *logger.ServerLogger
    IsProd  bool
    Modules *modules.Container
    Clients *intClients.ClientContainer
}
```

Individual servers are created from this base:

```go
base := NewBaseServer(log, isProd, clients, modules)
authServer := authServer.NewAuthServer(base)
bookingServer := bookingServer.NewBookingServer(base)
```

Handlers access services through the embedded modules:

```go
func (s *AuthServer) RefreshAuthTokens(ctx, req) (*Response, error) {
    token, err := s.Modules.AuthModule.JwtService.GetRefreshTokenFromCookie(ctx)
    // ...
}
```

This gives every handler access to all modules and all external clients through a single struct.

## External Clients Container

`ClientContainer` manages gRPC connections to peer services:

```go
type ClientContainer struct {
    RecService             recService.RecServiceClient
    HotelProviderService   hotelService.HotelProviderServiceClient
    BookingService         bookingService.BookingServiceClient
    // ... 10+ more
}
```

Each client is created with a `createXyzClient(log, config)` factory that:
- dials the gRPC address
- registers a health check client
- registers a connection closer

Health checks run against all clients during startup via `CheckAllHealth()`.

## Closers Pattern (Resource Lifecycle)

Resources are tracked with a `[]func() error` slice:

- On initialization, each resource registers a closer: `closers = append(closers, db.Close)`
- On shutdown, closers execute in **reverse order** (LIFO):

```go
func (a *App) close(ctx context.Context) {
    a.grpcServer.server.GracefulStop()
    a.modules.Close(ctx)
    for i := len(a.closers) - 1; i >= 0; i-- {
        _ = a.closers[i]()
    }
}
```

This ensures dependent resources close before their dependencies (e.g., services close before
DB connections).

## Repository Pattern

Repositories use gorm and support transactions:

```go
type RepositoryService struct {
    log  *logger.ServerLogger
    db   *gorm.DB
    UserRepository *UserRepository
}

func (r *RepositoryService) WithTrx() RepositoryService {
    return NewRepositoryService(r.log, r.db.Begin())
}

func (r *RepositoryService) Commit(ctx context.Context) error
func (r *RepositoryService) Rollback(ctx context.Context)
```

Notable: repositories and services are **concrete types**, not interfaces. This simplifies wiring
but makes unit testing harder (requires real DB or gorm mocking).

## Config Pattern

All configuration uses `sethvargo/go-envconfig` with struct tags:

```go
type AppConfig struct {
    Env          string `env:"ENV,required"`
    AdminToken   string `env:"ADMIN_TOKEN,required"`
    RedisConfig  RedisConfig
    StripeConfig StripeConfig
    // ... nested configs for each external service
}
```

Each module also has its own config struct loaded independently from environment.

## Key Characteristics

| Aspect | Approach |
|--------|----------|
| DI style | manual constructor chaining, no framework |
| Composition root | single `App` struct |
| Module isolation | each module owns its DB, repos, services |
| Interface usage | none at layer boundaries (concrete types) |
| Config loading | `go-envconfig` with struct tags |
| Resource cleanup | LIFO closers slice |
| Handler DI | embedded `BaseServer` with modules + clients |
| Transaction support | `WithTrx()` pattern on repository service |
| Health checks | gRPC health protocol on all external clients |
| Server architecture | gRPC-first with HTTP gateway overlay |

## Adaptation for URL Shortener

### What to adopt directly

**1. App struct as composition root with init chain**

Replace the current flat `main.go` that inlines all setup with an `App` struct in `internal/app/`:

```go
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
```

This keeps `cmd/api-http-stdlib/main.go` thin (just `NewApp` + `Run`) and makes the startup
sequence explicit and testable.

**2. LIFO closers pattern**

Collect cleanup functions during initialization, run them in reverse on shutdown:

```go
func (a *App) addCloser(fn func() error) {
    a.closers = append(a.closers, fn)
}

func (a *App) Shutdown(ctx context.Context) {
    a.srv.Shutdown(ctx)
    for i := len(a.closers) - 1; i >= 0; i-- {
        _ = a.closers[i]()
    }
}
```

**3. `go-envconfig` for config loading**

Replace the manual `os.Getenv` + `validate()` approach with `sethvargo/go-envconfig` struct tags:

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

Why switch:
- declarative: config shape and constraints visible at a glance from the struct definition
- less boilerplate: no manual `getEnvOrDefault`, no `strconv.Atoi`, no `validate()` method
- nested configs: as the project grows (Redis, Kafka, ClickHouse), compose config structs cleanly
- custom validation still possible via a post-load `Validate()` method for cross-field rules

The `.env` file convention stays the same: loaded by Docker Compose, not parsed by Go code.

**4. Constructor-based DI for services and handlers**

Wire dependencies via explicit constructor parameters, no DI framework:

```go
linkRepo := postgres.NewLinkRepository(pool)
linkService := service.NewLinkService(log, linkRepo)
linkHandler := handler.NewLinkHandler(log, linkService)
```

### What to adapt

**1. Interfaces at layer boundaries (differs from skibookers)**

Skibookers uses concrete types everywhere. The URL shortener should define interfaces at repository
and service boundaries to enable unit testing without infrastructure:

```go
// in internal/service/
type LinkRepository interface {
    Create(ctx context.Context, link *domain.Link) error
    GetByShortCode(ctx context.Context, code string) (*domain.Link, error)
}

type LinkService struct {
    log  *slog.Logger
    repo LinkRepository  // interface, not concrete type
}
```

This allows test doubles for repository calls without spinning up PostgreSQL.

**2. Direct handler injection (no BaseServer embedding)**

Skibookers embeds a `BaseServer` that gives every handler access to all modules and all clients.
The URL shortener is simpler — one HTTP binary with focused handlers. Pass only the needed
dependencies to each handler constructor:

```go
func NewLinkHandler(log *slog.Logger, svc *service.LinkService) *LinkHandler
func NewRedirectHandler(log *slog.Logger, svc *service.RedirectService) *RedirectHandler
```

This keeps handler dependencies explicit and avoids god-object access patterns.

**3. No module isolation yet**

Skibookers splits into UserModule, AuthModule, etc., each with its own DB. The URL shortener
shares a single PostgreSQL database. Module isolation becomes relevant when the worker (E05) and
analytics (E06) introduce ClickHouse and Kafka — until then, a flat service/repository structure
under `internal/` is sufficient.

**4. `log/slog` instead of `zap`**

Already decided in project docs. The logger constructor and context propagation pattern stay as
implemented.

### Target structure after refactor

```
cmd/api-http-stdlib/main.go    — thin: NewApp(ctx) + app.Run(ctx)
cmd/worker/main.go             — thin: NewWorkerApp(ctx) + app.Run(ctx)

internal/
├── app/
│   ├── app.go                 — App struct, init chain, Run, Shutdown
│   └── worker_app.go          — WorkerApp struct (simpler, no DB in E01)
├── config/
│   └── config.go              — go-envconfig struct tags
├── domain/link/               — Link entity (E02)
├── service/                   — business logic with interface deps
├── repository/postgres/       — concrete implementations
├── transport/http/stdlib/     — handlers, respond helper
└── platform/
    ├── logger/                — slog constructor, context helpers
    ├── health/                — live/ready handlers
    ├── metrics/               — placeholder
    └── middleware/             — request_id, logging, recovery, chain
```
