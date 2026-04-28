# Architecture Blueprint

A general, project-agnostic reference for building Go services using the same composition,
DI, and lifecycle patterns we use here. Copy this into a new project and replace the example
names with your domain.

This doc is the **how** of our structure. For the **why** and the historical comparison
with other patterns (lazy DI containers, framework-driven DI, flat `main.go`), see
`di_reference_skibookers.md`.

---

## 1. Core Principles

1. **Composition root pattern.** All wiring for a runtime happens in one place
   (`services/<service>/internal/app`). Nothing else in the codebase calls constructors of
   cross-layer dependencies.
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
8. **One service binary = one App.** Each runtime keeps its own `App` struct under
   `services/<service>/internal/app`. Shared init steps live in helper functions, not in a
   god-object.

---

## 2. Directory Layout

```
<project>/
├── services/
│   ├── shortener/
│   │   ├── cmd/
│   │   │   └── shortener/      # entry point: tiny main.go, just calls app.NewApp + Run
│   │   │       └── main.go
│   │   ├── Dockerfile          # service-owned image build
│   │   ├── migrations/
│   │   │   └── postgres/       # service-owned SQL migrations
│   │   └── internal/
│   │       ├── app/            # composition root for the shortener runtime
│   │       │   ├── app.go      # App struct, NewApp, Run, Shutdown, init* methods
│   │       │   ├── router.go   # buildRouter — HTTP table of contents
│   │       │   └── di.go       # container struct + newContainer wiring
│   │       ├── domain/         # service-owned business types and interfaces
│   │       ├── service/        # shortener business logic
│   │       ├── repository/     # shortener-owned store adapters
│   │       └── transport/      # entry-point packages, grouped by protocol then feature
│   │           └── http/
│   │               └── stdlib/         # net/http variant; siblings (chi/, echo/) may live here
│   │                   ├── respond/    # shared JSON response helpers
│   │                   ├── link/       # ONE feature = ONE package
│   │                   │   ├── handler.go   # Handler struct, NewHandler, Register(mux)
│   │                   │   ├── handlers.go  # handler methods (split when they grow)
│   │                   │   └── dto.go       # request/response shapes for this feature
│   │                   ├── redirect/
│   │                   │   ├── handler.go
│   │                   │   └── handlers.go
│   │                   └── admin/      # audience subgrouping when admin/public diverge
│   │                       └── link/
│   ├── identity/
│   │   ├── cmd/
│   │   │   ├── identity/              # HTTP runtime for accounts, sessions, OAuth, CSRF
│   │   │   └── identity-maintenance/  # one-shot operational CLI, e.g. retention cleanup
│   │   ├── Dockerfile
│   │   ├── migrations/
│   │   │   └── postgres/
│   │   └── internal/
│   │       ├── app/            # composition root for the identity HTTP runtime
│   │       ├── config/         # identity-owned env contract
│   │       ├── domain/         # accounts, auth identities, sessions, OAuth state
│   │       ├── service/        # auth, OAuth, retention, token/password helpers
│   │       ├── repository/     # identity-owned store adapters
│   │       └── transport/
│   │           └── http/
│   │               └── stdlib/
│   │                   ├── auth/       # auth/session/OAuth HTTP contract
│   │                   └── respond/
│   └── analytics/
│       ├── cmd/
│       │   └── analytics/
│       │       └── main.go
│       ├── Dockerfile          # service-owned image build
│       ├── migrations/         # add store-specific DDL only when the service owns schema
│       └── internal/
│           ├── app/            # analytics runtime wiring
│           └── config/         # analytics-owned env contract
├── internal/
│   └── platform/               # cross-cutting infra used by every binary
│       ├── logger/             # slog setup + context helpers
│       ├── middleware/         # request_id, logging, recovery, chain
│       ├── health/             # /health/live and /health/ready handlers
│       └── metrics/            # /metrics handler placeholder
│
├── deploy/
│   └── docker-compose/         # system-level orchestration only
│       └── docker-compose.yml
│
├── docs/                       # design and reference docs
├── ai/
│   ├── plan/                   # task board (epics + backlog/active/review/complete)
│   └── skills.md               # AI workflow and skill-memory doc
├── .env                        # local default values, loaded by docker-compose
├── Makefile                    # build, test, run, migrate-up, migrate-down
└── go.mod
```

### Layout rules

- **`services/<service>/internal/domain`** has no I/O imports — no `pgx`, no `http`, no `redis`. Only stdlib
  and small libs like `uuid`. Everything else depends on it.
- **`services/<service>/internal/service`** imports `domain` and stdlib. It depends on **interfaces** declared
  in `domain`, never on concrete repository types.
- **`services/<service>/internal/repository/<store>`** implements `domain.<Entity>Repository`. It is the only
  place that imports the store driver.
- **`services/<service>/internal/transport/<protocol>/<framework>/<feature>/`** is one package per
  feature. Each feature package owns a `Handler` struct, a `NewHandler` constructor, a `Register(mux)`
  method that wires its routes, handler methods, and its own DTOs. (We use `Handler` instead of
  `Server` because in HTTP "server" already means `http.Server` — `Server` is gRPC vocabulary.)
  Transport packages depend on narrow handler-local interfaces and must never call repositories.
  They may import service-layer transport-safe result types when that keeps the handler contract
  explicit; they should not construct concrete service implementations. See §3.4.
- **`internal/platform/...`** is cross-cutting infrastructure with no business knowledge.
- **`services/<service>/internal/app`** is the composition root for that runtime — the only
  place where concrete `service.New*` and `postgres.New*` are called.
- **`services/<service>/Dockerfile`** belongs to the service that owns the image. Keep image build
  logic near the runtime it builds.
- **`services/<service>/migrations/...`** belongs to the service that owns the schema. Keep
  service-owned SQL migrations out of repo-root catch-all folders.
- **A service may own more than one command** when the extra binary is operationally tied to the
  same domain, for example `cmd/identity` plus `cmd/identity-maintenance`. Keep long-running HTTP
  runtimes behind an `App`; keep one-shot maintenance commands explicit and small.
- **`deploy/...`** owns orchestration only. Compose, Helm, and cluster manifests may reference
  service Dockerfiles and migrations, but they do not own them.

---

## 3. Composition Root Pattern

### 3.1 The App struct

```go
// services/shortener/internal/app/app.go
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
- **Servers** — `srv`. Lifecycle endpoints (HTTP, gRPC, analytics loops).
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
// services/shortener/internal/app/di.go
type container struct {
    // Repositories — store-specific data access.
    <entity>Repo *postgres.<Entity>Repo

    // Services — business logic.
    <entity>Svc *service.<Entity>Service

    // HTTP feature handlers — one package per feature under transport/http/<framework>/.
    <entity>Handler *<entity>.Handler
}

func newContainer(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) (*container, error) {
    c := &container{}

    // Repositories.
    c.<entity>Repo = postgres.New<Entity>Repo(pool)

    // Services.
    c.<entity>Svc = service.New<Entity>Service(c.<entity>Repo, log)

    // HTTP feature handlers.
    c.<entity>Handler = <entity>.NewHandler(c.<entity>Svc, log, cfg.BaseURL)

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

### 3.4 Routing — feature packages + buildRouter + slim initHTTPServer

Routing is split across **three files** with strict responsibilities. The split exists so
that adding the 60th route does not turn `initHTTPServer` into a wall of `mux.HandleFunc`
calls and so that grepping for any feature lands in exactly one folder.

#### 3.4.1 The feature package — `<feature>/handler.go`

Each feature owns a package under `internal/transport/<protocol>/<framework>/<feature>/`.
That package exposes a `Handler` struct with its dependencies, a `NewHandler` constructor, and
a `Register(mux *http.ServeMux)` method that wires every route the feature serves:

```go
// internal/transport/http/stdlib/<feature>/handler.go
package <feature>

type Handler struct {
    svc     domain.<Entity>Service
    log     *slog.Logger
    baseURL string
}

func NewHandler(svc domain.<Entity>Service, log *slog.Logger, baseURL string) *Handler {
    return &Handler{svc: svc, log: log, baseURL: baseURL}
}

func (h *Handler) Register(mux *http.ServeMux) {
    mux.HandleFunc("POST   /api/v1/<entity>",              h.Create)
    mux.HandleFunc("GET    /api/v1/<entity>/{id}",         h.GetByID)
    mux.HandleFunc("POST   /api/v1/<entity>/{id}/disable", h.Disable)
    mux.HandleFunc("POST   /api/v1/<entity>/{id}/enable",  h.Enable)
    mux.HandleFunc("DELETE /api/v1/<entity>/{id}",         h.Delete)
}
```

Auth-style feature packages follow the same rule even when the surface is broader than CRUD:
`auth.Handler.Register` owns CSRF, password signup/login/logout, current-session validation, and
OAuth start/callback routes. Stable route strings, header names, cookie names, OAuth provider
names, error codes, and audit operation names should live in `const` blocks near the contract that
uses them.

Why `Handler` and not `Server`: in HTTP, "Server" already means `http.Server` (the listener);
`Server` is gRPC vocabulary where you literally implement an `XxxServer` interface from
generated protobuf code. In Go HTTP code, `Handler` is the standard term and the package
qualifier (`link.Handler`, `redirect.Handler`) keeps it unambiguous against `http.Handler`
the interface.

Handler methods live in `handlers.go`; request/response shapes live in `dto.go`. When a
feature grows past ~300 lines, split `handlers.go` into `handlers_crud.go`,
`handlers_bulk.go`, `handlers_stats.go` — same package, same `Handler`, same `Register`.

#### 3.4.2 The router — `app/router.go`

`buildRouter` is the **table of contents** for the binary's HTTP surface. It registers
platform routes directly, asks each feature handler to register its own routes, and wraps
everything in the middleware chain. Adding a new feature is one new line here.

```go
// services/<service>/internal/app/router.go
func (a *App) buildRouter() http.Handler {
    mux := http.NewServeMux()

    // Platform routes — owned by the binary itself.
    mux.Handle("GET /health/live",  health.LiveHandler())
    mux.Handle("GET /health/ready", health.ReadyHandler(func(ctx context.Context) error {
        return a.pool.Ping(ctx)
    }))
    mux.Handle("GET /metrics", metrics.Handler())

    // Feature handlers — each owns its own routes.
    a.di.<entity>Handler.Register(mux)
    a.di.redirectHandler.Register(mux)
    // a.di.adminLinkHandler.Register(mux)  // later

    // Cross-cutting middleware — first entry runs first at request time.
    return middleware.Compose(mux,
        middleware.Recovery,
        middleware.RequestID,
        middleware.CorrelationID,
        middleware.Logging(a.log),
    )
}
```

#### 3.4.3 initHTTPServer — pure server config

With routing extracted, `initHTTPServer` is reduced to building the `http.Server` struct.
It never touches a route, never touches a middleware, never calls a `New*`.

```go
func (a *App) initHTTPServer() {
    a.srv = &http.Server{
        Addr:         ":" + a.cfg.AppPort,
        Handler:      a.buildRouter(),
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
}
```

#### Why this split

- **Find anything in one folder.** All link-related HTTP code lives in `link/`. No more
  grepping a soup `handler/` package full of unrelated handlers.
- **No name collisions across features.** `link.CreateRequest` and `user.CreateRequest`
  coexist cleanly — Go's package boundary handles it.
- **Adding a feature is mechanical.** `mkdir <feature>/`, write `handler.go` + `handlers.go`
  + `dto.go`, add one field + one constructor line in `di.go`, add one `Register` call in
  `router.go`. Three files, three lines elsewhere. Done.
- **`router.go` stays readable forever.** It's a flat list of `Register` calls — the binary's
  HTTP surface fits on one screen no matter how many features you add.
- **Per-route or per-group middleware is trivial.** A future `/admin/*` subgroup builds its
  own sub-mux, registers admin servers on it, wraps with `middleware.Compose(subMux, Auth())`,
  and mounts it on the parent. No rewrite of unrelated routes.
- **Audience subgrouping by directory.** When admin and public APIs diverge,
  `transport/http/stdlib/admin/<feature>/` keeps them physically separated. The directory
  tree shows the API surface at a glance.
- **Multiple HTTP framework variants.** The `<framework>` segment (`stdlib/`, future `chi/`,
  `echo/`) lets you experiment with alternative routers without disturbing existing feature
  packages — only the `Register` method's mux type changes.

#### Anti-patterns

- ❌ One big `handler/` package containing every handler. Soups don't scale; name
  collisions force ugly prefixes.
- ❌ Route registration scattered across `init()` functions or package-level `var`s. Routes
  must be visible from `router.go`.
- ❌ Feature packages reaching into each other (`link` importing `user`). Shared logic
  belongs in `domain` or a cross-cutting helper, not in another feature package.
- ❌ A `BaseHandler` struct embedded into every feature handler to share deps. It hides what
  each handler actually needs — keep constructor injection per handler, even if it repeats
  three fields.
- ❌ Naming the feature type `Server`. That's gRPC vocabulary; in HTTP it collides with
  `http.Server`.
- ❌ A `New*` call inside `buildRouter` or `initHTTPServer`. Construction belongs in
  `newContainer`, not in routing or lifecycle code.

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
// services/shortener/cmd/shortener/main.go
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

Interfaces live in **`services/<service>/internal/domain`**, alongside the entities they describe. The
**consumer** declares the interface; the **producer** implements it. This matches the Go
idiom of "accept interfaces, return structs".

```go
// services/shortener/internal/domain/<entity>_repository.go
type <Entity>Repository interface {
    Create(ctx context.Context, e *<Entity>) error
    GetByID(ctx context.Context, id uuid.UUID) (*<Entity>, error)
    // ...
}

// services/shortener/internal/domain/<entity>_service.go
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

Sentinel errors live in `services/<service>/internal/domain/errors.go`:

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
// services/shortener/internal/config/config.go
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

- **One config package per service binary.** Keep env ownership local to the runtime that uses
  it; never hide service-specific settings in a repo-wide config package.
- **Do not force sibling services to share a config struct just because they live in one repo.**
  Shared env shape is an optimization, not a default.
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

Core rules:

- **Always JSON in non-local environments.**
- **Always include `service` and `env` as base attributes.**
- **Pass logger via constructor.** Components store it as a field; they never call
  `slog.Default()` and the package never exposes a singleton or `init()`-built global.
- **No `New(...)` calls in `init()`.** The logger is built in `app.go` from config so
  level/output can be overridden in tests and by env.
- **Always use the `*Context` variants** (`InfoContext`, `ErrorContext`, …) and pass
  `ctx` so request-scoped fields get attached automatically (see below).

#### Context-aware enrichment (the important part)

`logger.New` does **not** return a plain `slog.JSONHandler`. It wraps the JSON handler
in a small `ctxHandler` that implements `slog.Handler` and reads request-scoped
attributes (`request_id`, `correlation_id`, `trace_id`) from `ctx` inside `Handle`:

```go
type ctxHandler struct{ slog.Handler }

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
    if attrs := requestmeta.AttrsFromContext(ctx); len(attrs) > 0 {
        r.AddAttrs(attrs...)
    }
    return h.Handler.Handle(ctx, r)
}

func (h *ctxHandler) WithAttrs(as []slog.Attr) slog.Handler {
    return &ctxHandler{Handler: h.Handler.WithAttrs(as)}
}
func (h *ctxHandler) WithGroup(name string) slog.Handler {
    return &ctxHandler{Handler: h.Handler.WithGroup(name)}
}
```

Why this matters:

- Call sites are clean — handlers and services just write
  `h.log.InfoContext(ctx, "link created", "link_id", id)`. There is **no**
  `log := EnrichFromContext(ctx, h.log)` ritual at the top of every method.
- Enrichment runs **after** level filtering, so dropped logs pay zero cost. `With()`
  decoration is preserved because `WithAttrs`/`WithGroup` re-wrap.
- Adding new request fields (e.g. OTel `trace_id`, `tenant_id`, `user_id`) is a
  **one-line change in `requestmeta.AttrsFromContext`** — no edits at any call site.
- No `zap.AddCallerSkip` games, no per-method wrapper functions, no global singleton.

#### `WithContext` / `FromContext`

Reserved for code paths that don't have a logger field at hand (e.g. helpers shared
between an HTTP handler and a worker job that want to bind a long-lived attribute like
`job_id` to the logger and have downstream callees pick it up). Most code should
**not** need `FromContext` — prefer the constructor-injected `*slog.Logger`.

#### Anti-patterns we explicitly avoid

- ❌ Package-level `init()` that builds a logger from `os.Getenv` directly.
- ❌ `GetLogger()` global accessor — hides the dependency, can't be overridden in tests.
- ❌ Wrapper types like `ServerLogger` with custom `Info(ctx, msg, fields...)` that
  re-implement what a context-aware `slog.Handler` already does.
- ❌ `zap.AddCallerSkip(N)` to compensate for wrapper layers — fragile, breaks the
  moment a helper is added.
- ❌ Calling `EnrichFromContext` (or any equivalent) at the top of every handler/service
  method. If you find yourself writing that, the enrichment belongs in the handler.

### 6.2 Middleware chain

Standard order, applied in `initHTTPServer`:

```
recovery → request_id → correlation_id → telemetry → metrics → logging → handler
```

- **`recovery`** catches panics, logs with stack, returns standard JSON error.
- **`request_id`** reads `X-Request-ID` or generates one, stores in context, sets on
  response header.
- **`correlation_id`** reads `X-Correlation-ID` or falls back to `request_id`.
- **`telemetry`** starts a server span and attaches trace metadata to context.
- **`metrics`** records HTTP duration and status labels.
- **`logging`** emits one structured line per request with method, path, status, duration,
  request_id, correlation_id, and trace_id when available.

### 6.3 Health and metrics ports

| Binary | Port | Purpose |
|--------|------|---------|
| Shortener | `APP_PORT` (e.g. 8080) | serves both business routes AND `/health/*`, `/metrics` |
| Identity | `IDENTITY_APP_PORT` (e.g. 8081) | serves auth/session/OAuth routes AND `/health/*`, `/metrics` |
| Analytics | `ANALYTICS_ADMIN_PORT` (e.g. 9090) | serves only `/health/*`, `/metrics` |

The analytics runtime has no user-facing write API but still exposes a small admin/stats server so
Kubernetes probes and Prometheus scraping work uniformly. Identity follows the same health and
metrics shape as shortener so Compose, Prometheus, and future gateway validation can treat it as a
first-class backend service.

---

## 7. Multiple Binaries Pattern

A typical runtime split here has:

- `services/shortener/cmd/shortener` — request-driven, owns the HTTP API
- `services/identity/cmd/identity` — request-driven, owns account, session, CSRF, and OAuth API
- `services/identity/cmd/identity-maintenance` — one-shot operational command for identity
  retention cleanup
- `services/analytics/cmd/analytics` — background-driven, owns timers/queues/consumers

Each long-running binary has:

- its own `App` struct
- its own `container` when that runtime needs one
- its own `NewApp`
- shared init helpers via package-level functions in `services/<service>/internal/app/` or a
  shared infra package when useful

One-shot maintenance commands can skip the `App` struct when they only load config, open one
resource, execute one service method, and exit. They should still reuse the same service and
repository layers as the HTTP runtime.

### Why not one big container with everything?

- The API binary should not import Kafka producer code if it doesn't use it.
- The analytics runtime should not import HTTP handlers.
- The identity maintenance command should not import HTTP handlers or start an HTTP server.
- Each runtime's `container` reflects only what that runtime needs, so build size and failure
  surface stay minimal.

### Sharing init logic

If both binaries need the same DB pool setup, extract a helper:

```go
// internal/platform/db.go
func openPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) { ... }
```

Both runtime apps can call it. The helper does not own a closer — the
binary that calls it registers cleanup.

---

## 8. Migrations

- **Tool**: `golang-migrate`.
- **Layout**: `services/<service>/migrations/postgres/NNNNNN_description.{up,down}.sql`.
- **Rule**: migrations are applied **explicitly**, never on app boot. The application
  process should fail (or refuse to serve) if the schema is missing required tables, but
  it should not run migrations itself.
- **Metadata**: when multiple service-owned migration streams share one physical database during
  local bootstrap, give each stream its own migrate metadata table (for example
  `schema_migrations_identity`). Independent services may each start at `000001`; a shared
  metadata table would incorrectly make one service's first migration hide another's.
- **Operational**:
  - local: `make migrate-up`
  - service-specific local streams: `make <service>-migrate-up` or `task <service>:migrate`
  - CI: run before integration tests
  - deploy: run before switching traffic

See `migrations.md` for the full operational guide.

## 8.1 Dockerfiles And Deploy Ownership

- **Service Dockerfiles live with the service**: `services/<service>/Dockerfile`.
- **Deploy manifests live under `deploy/`** and point at those Dockerfiles or published images.
- **Do not hide service build definitions under `deploy/`** unless the Dockerfile is truly
  shared infra tooling rather than a service image.

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

A lightweight project board lives in `ai/plan/`:

```
ai/plan/
├── 0-epics/                # active epics (plus 0-epics/completed/ for finished ones)
├── 1-backlog/              # tasks not started
├── 2-active/               # tasks in progress
├── 3-review/               # tasks awaiting validation
├── 4-complete/             # finished tasks (lightweight history)
├── templates/              # epic.md and task.md templates
└── README.md               # workflow rules
```

Tasks move between folders by `git mv`. The filename stays the same so history is easy to
follow. See `ai/plan/README.md` for full conventions.

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
3. **Write `services/<service>/internal/config/config.go`** with one struct and `Load(ctx)`.
4. **Write `internal/platform/logger`** with `New(level, service, env)`.
5. **Write `internal/platform/middleware`** with `recovery`, `request_id`, `logging`,
   `Chain`.
6. **Write `internal/platform/health` and `internal/platform/metrics`** placeholders.
7. **Write `services/<service>/internal/app/app.go`** with the `App` struct, `NewApp`,
   `Run`, `Shutdown`, `initConfig`, `initLogger`, `initDB`, `initHTTPServer`. Leave
   `initContainer` empty for now.
8. **Write `services/<service>/internal/app/di.go`** with an empty `container` struct and
   `newContainer`.
9. **Write `services/<service>/cmd/<binary>/main.go`** as the 5-line entry point.
10. **Add `services/<service>/migrations/postgres/000001_*.sql`** for the first table.
11. **Add `Makefile` targets**: `build`, `test`, `lint`, `run`, `stop`, `migrate-up`,
    `migrate-down`, `migrate-create`.
12. **Add `deploy/docker-compose/docker-compose.yml`** with `postgres`, your service binary,
    and a one-shot `migrate` service under the `tools` profile.
13. **Add the first domain entity** following the pattern in section 4: struct → repo
    interface → service interface → postgres repo → service impl → handler → DTOs.
14. **Wire it in `newContainer`**, register routes in `initHTTPServer`.
15. **Add maintenance commands only when a bounded operational workflow exists** (retention,
    repair, one-shot import). Reuse service/repository code; do not add a background worker by
    default.
16. **Run `make migrate-up && make run` and curl your endpoint.**
17. **Set up the plan board** by copying `ai/plan/templates/` and `ai/plan/README.md`.

If steps 1–12 take more than an afternoon, the blueprint is fighting you and we should
revisit it.
