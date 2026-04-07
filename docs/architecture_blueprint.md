# Architecture Blueprint

A general, project-agnostic reference for building Go services using the same composition,
DI, and lifecycle patterns we use here. Copy this into a new project and replace the example
names with your domain.

This doc is the **how** of our structure. For the **why** and the historical comparison
with other patterns (lazy DI containers, framework-driven DI, flat `main.go`), see
`di_reference_skibookers.md`.

---

## 1. Core Principles

1. **Composition root pattern.** All wiring happens in one place (`internal/app`). Nothing
   else in the codebase calls constructors of cross-layer dependencies.
2. **Constructor injection only.** No service locator, no global singletons, no init() side
   effects, no reflection-based DI framework. If a type needs `X`, it takes `X` as a
   constructor argument.
3. **Eager initialization.** All dependencies are built at startup, not on first request.
   Boot fails fast and visibly; production traffic never hits a half-built system.
4. **Errors propagate, processes don't `os.Exit` deep in the stack.** Construction errors
   flow up through `NewApp` to `main`, where logging and exit codes are handled in one
   place.
5. **Context-aware init.** `context.Context` flows into every constructor that does I/O so
   timeouts and cancellation work from the very first connection attempt.
6. **LIFO shutdown.** Resources are closed in reverse construction order, owned by the
   composition root, never by the components themselves.
7. **Interfaces at boundaries, concrete types in the composition root.** Layers depend on
   interfaces; the wiring code holds concrete pointers and is the only place that knows
   the implementation.
8. **One binary = one App.** Each `cmd/<binary>` has its own `App` struct in `internal/app`.
   Shared init steps live in helper functions, not in a god-object.

---

## 2. Directory Layout

```
<project>/
├── cmd/
│   ├── api-http-stdlib/        # entry point: tiny main.go, just calls app.NewApp + Run
│   │   └── main.go
│   └── worker/                 # second binary, same shape
│       └── main.go
│
├── internal/
│   ├── app/                    # composition root — owned by the binaries
│   │   ├── app.go              # App struct, NewApp, Run, Shutdown, init* methods
│   │   ├── di.go               # container struct + newContainer wiring
│   │   └── worker_app.go       # WorkerApp + worker container (for the second binary)
│   │
│   ├── config/                 # env-driven config struct + Load function
│   │   └── config.go
│   │
│   ├── domain/                 # business types and interfaces (no I/O code)
│   │   ├── <entity>.go         # struct
│   │   ├── errors.go           # sentinel errors (ErrNotFound, ...)
│   │   ├── <entity>_repository.go  # interface — what services need from storage
│   │   └── <entity>_service.go     # interface — what handlers need from business logic
│   │
│   ├── service/                # business logic — orchestrates repositories
│   │   ├── <entity>_service.go
│   │   └── <entity>_service_test.go   # uses mock repository
│   │
│   ├── repository/
│   │   └── postgres/           # one subdir per store; clickhouse/, redis/, kafka/ later
│   │       └── <entity>_repo.go
│   │
│   ├── transport/
│   │   └── http/
│   │       └── stdlib/
│   │           ├── handler/    # HTTP handlers + DTOs
│   │           │   ├── <entity>_handler.go
│   │           │   └── dto.go
│   │           └── respond/    # JSON response helpers (success + error shape)
│   │
│   └── platform/               # cross-cutting infra used by every binary
│       ├── logger/             # slog setup + context helpers
│       ├── middleware/         # request_id, logging, recovery, chain
│       ├── health/             # /health/live and /health/ready handlers
│       └── metrics/            # /metrics handler placeholder
│
├── migrations/
│   └── postgres/               # NNNNNN_description.{up,down}.sql
│
├── deploy/
│   └── docker-compose/
│       ├── docker-compose.yml
│       ├── Dockerfile.api
│       └── Dockerfile.worker
│
├── docs/                       # design and reference docs
├── plan/                       # task board (epics + backlog/active/review/complete)
├── .env                        # local default values, loaded by docker-compose
├── Makefile                    # build, test, run, migrate-up, migrate-down
└── go.mod
```

### Layout rules

- **`internal/domain`** has no I/O imports — no `pgx`, no `http`, no `redis`. Only stdlib
  and small libs like `uuid`. Everything else depends on it.
- **`internal/service`** imports `domain` and stdlib. It depends on **interfaces** declared
  in `domain`, never on concrete repository types.
- **`internal/repository/<store>`** implements `domain.<Entity>Repository`. It is the only
  place that imports the store driver.
- **`internal/transport/...`** implements HTTP, gRPC, or other entry points. It depends on
  `domain` interfaces, never directly on `service` or `repository` concrete types.
- **`internal/platform/...`** is cross-cutting infrastructure with no business knowledge.
- **`internal/app`** is the only package that imports everything. It is the **composition
  root** — the only place where concrete `service.New*` and `postgres.New*` are called.

---

## 3. Composition Root Pattern

### 3.1 The App struct

```go
// internal/app/app.go
type App struct {
    cfg     *config.Config
    log     *slog.Logger
    pool    *pgxpool.Pool
    di      *container
    srv     *http.Server
    closers []func() error
}
```

Fields are grouped by purpose:
- **Config and observability** — `cfg`, `log`. Built first, used by everything else.
- **Infrastructure clients** — `pool` (and later `redis`, `kafka`, `clickhouse`). Long-lived
  resources that need cleanup.
- **Container** — `di`, the wired object graph for this binary.
- **Servers** — `srv`. Lifecycle endpoints (HTTP, gRPC, worker loops).
- **Closers** — LIFO list of cleanup functions registered as resources are built.

### 3.2 NewApp — strict ordered init

```go
func NewApp(ctx context.Context) (*App, error) {
    a := &App{}

    if err := a.initConfig(ctx); err != nil {
        return nil, fmt.Errorf("init config: %w", err)
    }
    a.initLogger()

    if err := a.initDB(ctx); err != nil {
        return nil, fmt.Errorf("init db: %w", err)
    }
    if err := a.initContainer(); err != nil {
        return nil, fmt.Errorf("init container: %w", err)
    }
    a.initHTTPServer()

    return a, nil
}
```

Rules:
- **Each step is its own method** so `NewApp` reads as a script.
- **Each fallible step returns an error**, wrapped with a step name for diagnosis.
- **Order matters and is explicit.** No lazy "auto-resolve" — you can read top-to-bottom
  and know exactly what is initialized when.
- **`initLogger` cannot fail.** Once config is loaded, building a `slog.Logger` is pure
  function call.
- **`ctx` flows into every I/O step.** Drivers like `pgxpool.NewWithConfig(ctx, ...)`
  honor cancellation and timeouts during their first connection attempt.

### 3.3 The container — graph in one file

```go
// internal/app/di.go
type container struct {
    // Repositories — store-specific data access.
    <entity>Repo *postgres.<Entity>Repo

    // Services — business logic.
    <entity>Svc *service.<Entity>Service

    // Handlers — HTTP transport.
    <entity>Handler *handler.<Entity>Handler
}

func newContainer(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) (*container, error) {
    c := &container{}

    // Repositories.
    c.<entity>Repo = postgres.New<Entity>Repo(pool)

    // Services.
    c.<entity>Svc = service.New<Entity>Service(c.<entity>Repo, log)

    // Handlers.
    c.<entity>Handler = handler.New<Entity>Handler(c.<entity>Svc, log, cfg.BaseURL)

    return c, nil
}
```

Why a separate `container`:
- `initHTTPServer` becomes **routing-only** — no `New*` calls in the same function as
  `mux.HandleFunc`.
- The wiring is in **one file**, grouped by layer, so the graph is visible at a glance.
- Adding a new component is mechanical: **add a field, add a constructor line, reference
  it from a route or another component.** Never touch lifecycle code.
- Returning `error` future-proofs it for components that can fail at construction
  (Redis client, Kafka producer, ClickHouse pool, third-party SDKs).

### 3.4 initHTTPServer — routing only

```go
func (a *App) initHTTPServer() {
    mux := http.NewServeMux()

    // Platform.
    mux.Handle("GET /health/live",  health.LiveHandler())
    mux.Handle("GET /health/ready", health.ReadyHandler(...))
    mux.Handle("GET /metrics",      metrics.Handler())

    // Domain routes.
    mux.HandleFunc("POST /api/v1/<entity>",      a.di.<entity>Handler.Create)
    mux.HandleFunc("GET /api/v1/<entity>/{id}",  a.di.<entity>Handler.GetByID)
    // ...

    chained := middleware.Chain(a.log, mux)

    a.srv = &http.Server{
        Addr:         ":" + a.cfg.AppPort,
        Handler:      chained,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
}
```

This function should **never contain a `New*` call** for a domain component. If you find
yourself adding one, it belongs in `newContainer`.

### 3.5 LIFO closers

```go
func (a *App) addCloser(fn func() error) {
    a.closers = append(a.closers, fn)
}

func (a *App) initDB(ctx context.Context) error {
    // ... build pool ...
    a.pool = pool
    a.addCloser(func() error { pool.Close(); return nil })
    return nil
}

func (a *App) Shutdown() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := a.srv.Shutdown(ctx); err != nil {
        a.log.Error("http shutdown error", "error", err)
    }

    for i := len(a.closers) - 1; i >= 0; i-- {
        if err := a.closers[i](); err != nil {
            a.log.Error("closer error", "error", err)
        }
    }
}
```

Rules:
- **Each fallible-construction step that creates a resource registers a closer immediately
  after the resource is built.** Never delegate cleanup to component code.
- **HTTP server is shut down first** (drain in-flight requests), **then** infrastructure
  closes in reverse construction order.
- **Closers swallow errors and log them** — shutdown is best-effort; you cannot abort it.

### 3.6 Run loop

```go
func (a *App) Run(ctx context.Context) {
    ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go func() {
        a.log.Info("server starting", "port", a.cfg.AppPort)
        if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            a.log.Error("server error", "error", err)
        }
    }()

    <-ctx.Done()
    a.log.Info("shutdown signal received")
    a.Shutdown()
}
```

### 3.7 main.go — tiny

```go
// cmd/api-http-stdlib/main.go
func main() {
    ctx := context.Background()
    a, err := app.NewApp(ctx)
    if err != nil {
        log.Fatalf("failed to init app: %v", err)
    }
    a.Run(ctx)
}
```

`main.go` is intentionally trivial. It is not a place for wiring; it is the OS entry
point. All logic lives in `app.NewApp`.

---

## 4. Layer Boundaries And Interfaces

### 4.1 Where interfaces live

Interfaces live in **`internal/domain`**, alongside the entities they describe. The
**consumer** declares the interface; the **producer** implements it. This matches the Go
idiom of "accept interfaces, return structs".

```go
// internal/domain/<entity>_repository.go
type <Entity>Repository interface {
    Create(ctx context.Context, e *<Entity>) error
    GetByID(ctx context.Context, id uuid.UUID) (*<Entity>, error)
    // ...
}

// internal/domain/<entity>_service.go
type <Entity>Service interface {
    Create(ctx context.Context, input Create<Entity>Input) (*<Entity>, error)
    GetByID(ctx context.Context, id uuid.UUID) (*<Entity>, error)
    // ...
}
```

### 4.2 What each layer depends on

| Layer        | Depends on (interface)              | Returns         | Tested with                  |
|--------------|-------------------------------------|-----------------|------------------------------|
| Repository   | nothing (it is the leaf)            | concrete struct | integration test against DB  |
| Service      | `domain.<Entity>Repository`         | concrete struct | unit test with mock repo     |
| Handler      | `domain.<Entity>Service`            | concrete struct | unit test with mock service or integration against real handler |
| Container    | concrete types from all layers      | `*container`    | integration test if at all   |

The **container holds concrete pointers**, not interfaces. The composition root knows the
implementation; everywhere else depends on the abstraction.

### 4.3 Domain errors

Sentinel errors live in `internal/domain/errors.go`:

```go
var (
    ErrNotFound      = errors.New("entity not found")
    ErrAlreadyExists = errors.New("entity already exists")
    // ...
)
```

Repositories return them, services pass them through, handlers map them to HTTP status
codes via `errors.Is`. No layer-specific error wrappers leak past their boundary.

---

## 5. Configuration

### 5.1 Single struct, declarative tags

```go
// internal/config/config.go
type Config struct {
    AppPort    string `env:"APP_PORT,default=8080"`
    LogLevel   string `env:"LOG_LEVEL,default=INFO"`
    Env        string `env:"APP_ENV,default=local"`
    DBDSN      string `env:"DB_DSN,required"`
    DBMaxConns int32  `env:"DB_MAX_CONNS,default=10"`
    BaseURL    string `env:"BASE_URL,default=http://localhost:8080"`
}

func Load(ctx context.Context) (*Config, error) {
    cfg := &Config{}
    if err := envconfig.Process(ctx, cfg); err != nil {
        return nil, fmt.Errorf("config: %w", err)
    }
    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("config validation: %w", err)
    }
    return cfg, nil
}
```

### 5.2 Rules

- **One config struct per binary** (or one shared if both binaries take the same env).
  Never package-level globals.
- **`Load(ctx)` is the only public function.** No `MustLoad`, no `init()`.
- **Validation lives in `Validate()`** for cross-field rules that struct tags can't express.
- **Defaults belong in struct tags**, not in code paths that read the field.
- **Secrets and config use the same mechanism in dev**, but in production secrets come
  from a secret store and are injected as env vars at runtime.

---

## 6. Observability

### 6.1 Logger

```go
// internal/platform/logger/logger.go
func New(level, service, env string) *slog.Logger { ... }
func WithContext(ctx context.Context, l *slog.Logger) context.Context { ... }
func FromContext(ctx context.Context) *slog.Logger { ... }
```

- **Always JSON in non-local environments.**
- **Always include `service` and `env` as base attributes.**
- **Pass logger via constructor.** Components store it as a field; they never call
  `slog.Default()`.
- **Log lines flow through `slog.LogAttrs` or `Info(ctx, msg, "key", val)` form** for
  consistent structure.

### 6.2 Middleware chain

Standard order, applied in `initHTTPServer`:

```
recovery → request_id → logging → handler
```

- **`recovery`** catches panics, logs with stack, returns standard JSON error.
- **`request_id`** reads `X-Request-ID` or generates one, stores in context, sets on
  response header.
- **`logging`** emits one structured line per request with method, path, status, duration,
  request_id.

### 6.3 Health and metrics ports

| Binary | Port | Purpose |
|--------|------|---------|
| API    | `APP_PORT` (e.g. 8080) | serves both business routes AND `/health/*`, `/metrics` |
| Worker | `WORKER_ADMIN_PORT` (e.g. 9090) | serves only `/health/*`, `/metrics` |

The worker has no business HTTP API but still exposes a small admin server so Kubernetes
probes and Prometheus scraping work uniformly.

---

## 7. Two Binaries Pattern

A typical service has at least:
- `cmd/api-http-stdlib` — request-driven, owns the HTTP API
- `cmd/worker` — background-driven, owns timers/queues/consumers

Each binary has:
- its own `App` struct (`App` and `WorkerApp`)
- its own `container` (`container` and `workerContainer`)
- its own `NewApp` / `NewWorkerApp`
- shared init helpers via package-level functions in `internal/app/` if useful

### Why not one big container with both?

- The API binary should not import Kafka producer code if it doesn't use it.
- The worker should not import HTTP handlers.
- Each binary's `container` reflects only what that binary needs, so build size and
  failure surface stay minimal.

### Sharing init logic

If both binaries need the same DB pool setup, extract a helper:

```go
// internal/app/db.go
func openPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) { ... }
```

Both `App.initDB` and `WorkerApp.initDB` call it. The helper does not own a closer — the
binary that calls it registers cleanup.

---

## 8. Migrations

- **Tool**: `golang-migrate`.
- **Layout**: `migrations/postgres/NNNNNN_description.{up,down}.sql`.
- **Rule**: migrations are applied **explicitly**, never on app boot. The application
  process should fail (or refuse to serve) if the schema is missing required tables, but
  it should not run migrations itself.
- **Operational**:
  - local: `make migrate-up`
  - CI: run before integration tests
  - deploy: run before switching traffic

See `migrations.md` for the full operational guide.

---

## 9. Testing Strategy

| Layer       | Test type                      | What it covers |
|-------------|--------------------------------|----------------|
| Domain      | none usually (pure types)      | — |
| Service     | unit with mock repository      | business logic, error mapping, retry loops |
| Repository  | integration against real DB    | SQL correctness, constraint handling |
| Handler     | unit with mock service OR integration through real `httptest` server | request parsing, status codes, error JSON |
| App / wiring| integration smoke test         | "binary boots and responds to /health/live" |

### Mock pattern

A small hand-written mock per interface:

```go
type mock<Entity>Repo struct {
    createFn  func(ctx context.Context, e *domain.<Entity>) error
    getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.<Entity>, error)
}
func (m *mock<Entity>Repo) Create(ctx context.Context, e *domain.<Entity>) error { return m.createFn(ctx, e) }
// ...
```

**Do not pull in `gomock`/`mockery` for a project this size.** The cost of a generated
mock is not paid back. If a mock surface gets large, it is a smell that the interface is
too wide.

---

## 10. Plan Board

A lightweight project board lives in `plan/`:

```
plan/
├── 0-epics/                # active epics (plus 0-epics/completed/ for finished ones)
├── 1-backlog/              # tasks not started
├── 2-active/               # tasks in progress
├── 3-review/               # tasks awaiting validation
├── 4-complete/             # finished tasks (lightweight history)
├── templates/              # epic.md and task.md templates
└── README.md               # workflow rules
```

Tasks move between folders by `git mv`. The filename stays the same so history is easy to
follow. See `plan/README.md` for full conventions.

This is **not** a substitute for a real issue tracker on a team — it is a way to keep
implementation intent inside the repo so a future session (human or AI) can resume work
without chat history.

---

## 11. What This Pattern Deliberately Avoids

- **DI frameworks** (Wire, Fx, Dig). Reflection/codegen is overkill for the dependency
  count of a typical service. Constructor injection scales to ~50 components without help.
- **Lazy DI containers with `os.Exit`**. Too many failure modes hidden inside getters; you
  lose ctx, error returns, and testability.
- **Package-level globals** for config, logger, or DB connections. They make tests
  unparallelizable and hide dependencies.
- **`init()` functions for anything that has side effects.** The order of `init()` across
  packages is opaque; we want explicit order.
- **Generated mocks** for small interfaces. Hand-written mocks keep test code obvious.
- **Repositories that do business logic** or services that build SQL. Layers stay strict.
- **Handlers that talk to repositories directly.** Always go through a service.

---

## 12. Checklist For A New Project

When starting a new service from this blueprint:

1. **Create the layout** from section 2.
2. **Add `go.mod`**, then `pgx`, `go-envconfig`, `google/uuid` as the first three deps.
3. **Write `internal/config/config.go`** with one struct and `Load(ctx)`.
4. **Write `internal/platform/logger`** with `New(level, service, env)`.
5. **Write `internal/platform/middleware`** with `recovery`, `request_id`, `logging`,
   `Chain`.
6. **Write `internal/platform/health` and `internal/platform/metrics`** placeholders.
7. **Write `internal/app/app.go`** with the `App` struct, `NewApp`, `Run`, `Shutdown`,
   `initConfig`, `initLogger`, `initDB`, `initHTTPServer`. Leave `initContainer` empty
   for now.
8. **Write `internal/app/di.go`** with an empty `container` struct and `newContainer`.
9. **Write `cmd/<binary>/main.go`** as the 5-line entry point.
10. **Add `migrations/postgres/000001_*.sql`** for the first table.
11. **Add `Makefile` targets**: `build`, `test`, `lint`, `run`, `stop`, `migrate-up`,
    `migrate-down`, `migrate-create`.
12. **Add `deploy/docker-compose/docker-compose.yml`** with `postgres`, your binary, and
    a one-shot `migrate` service under the `tools` profile.
13. **Add the first domain entity** following the pattern in section 4: struct → repo
    interface → service interface → postgres repo → service impl → handler → DTOs.
14. **Wire it in `newContainer`**, register routes in `initHTTPServer`.
15. **Run `make migrate-up && make run` and curl your endpoint.**
16. **Set up the plan board** by copying `plan/templates/` and `plan/README.md`.

If steps 1–12 take more than an afternoon, the blueprint is fighting you and we should
revisit it.
