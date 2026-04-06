# E01 - Platform Bootstrap

## Summary

Foundation epic for bringing the service to a runnable local baseline with clear runtime boundaries.

## Goal

Start the `api-http-stdlib` application locally with PostgreSQL and Redis, validated config,
structured logging, basic middleware, graceful shutdown, and health or metrics endpoints.

## Scope

- initial repository and package layout for a modular monolith
- decide on `go.mod` module path (flat `sandbox_url_shortener` or qualified
  `github.com/<owner>/sandbox_url_shortener`)
- `cmd/api-http-stdlib` bootstrap and shared runtime wiring
- scaffold `cmd/worker` with a placeholder `main.go` and health/metrics HTTP server only (no Kafka
  consumer logic until E05)
- config loader with env-driven values validated on startup; default values in a `.env` file loaded
  by Docker Compose
- `log/slog` JSON logger with `request_id` context propagation via `slog.With()` attached in
  middleware and carried through `context.Context`
- `request_id` middleware: reuse incoming `X-Request-ID` or generate one, set in context and
  response header
- JSON error response helper implementing the standard shape defined in `system_design.md` API
  Response Conventions (`code`, `message`, `request_id`)
- basic middleware stack: request logging, panic recovery, request_id
- health (`/health/live`, `/health/ready`) and metrics (`/metrics`) endpoints
- graceful shutdown for both binaries
- Docker Compose stack for `api-http-stdlib + worker + PostgreSQL + Redis`; Redis is included for
  infrastructure readiness but the application does not connect to Redis until E03
- minimal Makefile with `build`, `run`, `test`, and `migrate-up` targets
- excludes link business logic, UI flows, and async analytics pipeline

## Key Decisions

- keep the system a modular monolith at bootstrap
- stay Docker-first for local development
- use `cmd/api-http-stdlib` as the canonical HTTP entrypoint for the MVP
- `shortener-service` listens on a single port (default `8080`) for API, health, and metrics
- `worker` exposes a dedicated admin port (default `9090`) for health and metrics only
- use structured JSON logs from the first runnable version
- use Go `log/slog` with JSON handler to `stdout`
- propagate `request_id` through `context.Context` using `slog.With()` in middleware so all
  downstream log calls include it automatically
- keep runtime configuration env-driven and validated on startup
- default env values live in a `.env` file at the project root, loaded by Docker Compose; the
  application reads from the OS environment only (no `.env` parsing in Go code)
- use `golang-migrate` with SQL migrations under `migrations/postgres`; migration files are created
  in E02 when the `links` schema is defined
- scaffold `cmd/worker` early to avoid a bootstrap detour when E05 begins

## Task Links

- `E01-001-establish-repo-layout-and-api-entrypoint.md`
- `E01-002-wire-config-logging-and-middleware.md`
- `E01-003-add-docker-compose-health-and-metrics-baseline.md`
- `E01-004-refactor-to-app-struct-and-envconfig.md`

## Resolved Questions

- worker binary is scaffolded in E01 with health/metrics only; Kafka consumer logic is added in E05
