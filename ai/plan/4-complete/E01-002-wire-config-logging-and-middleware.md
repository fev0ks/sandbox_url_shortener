# E01-002 - Wire Config, Logging, And Middleware

Status: complete

Epic: E01 - Platform Bootstrap

## Summary

Add the config loader, structured logger, and core middleware stack so both binaries start with
validated configuration, JSON logs, and request-level observability from the first real run.

## Scope

- config loader in `internal/config/` reading from OS environment with validation
- `.env` file at the project root with default development values
- `log/slog` JSON logger in `internal/platform/logger/`
- `request_id` middleware with `X-Request-ID` reuse/generation and context propagation
- logger context propagation via `slog.With()` stored in context
- request logging middleware: one JSON summary line per request
- panic recovery middleware returning standard JSON error shape
- JSON error response helper in `internal/transport/http/stdlib/respond/`
- middleware stack wired into both binaries

## Acceptance Criteria

- application starts with validated config; missing required env var causes a clear startup error
- every HTTP request produces exactly one structured JSON log line to stdout
- every response includes an `X-Request-ID` header
- panic in a handler returns `500` with the JSON error shape and logs the stack at `ERROR`
- `go test ./internal/config/...` passes with at least one test for missing required var
- `go test ./internal/platform/...` passes with at least one test for request_id propagation

## Assumptions

- required config fields for E01: `APP_PORT`, `WORKER_ADMIN_PORT`, `LOG_LEVEL`, `DB_DSN`
- `LOG_LEVEL` defaults to `INFO`; supports `DEBUG`, `INFO`, `WARN`, `ERROR`
- UUID generation for `request_id` uses `crypto/rand`

## Implementation Notes

- config loader: `Config` struct with explicit `validate()` method; returns error on startup if
  required fields missing or log level invalid; `DBMaxConns` parsed from `DB_MAX_CONNS` with
  default `10`
- logger: `New(level, service, env)` returns `*slog.Logger` with JSON handler to stdout;
  `WithContext`/`FromContext` for context propagation
- middleware ordering: `recovery -> request_id -> logging -> handler` via `Chain()` helper
- error helper: `respond.Error()` writes `{"error": {"code", "message", "request_id"}}`;
  `respond.JSON()` for success responses
- both binaries use the same `middleware.Chain()` wrapper

## Progress Checklist

- [x] implement config loader with validation
- [x] create `.env` with development defaults
- [x] implement slog JSON logger constructor
- [x] implement request_id middleware with context propagation
- [x] implement request logging middleware
- [x] implement panic recovery middleware
- [x] implement JSON error response helper
- [x] wire middleware into both entrypoints
- [x] add unit tests for config, request_id, and error helper
- [x] run `go test ./...`

## Validation

- `go test ./...` — 8 tests across 3 packages, all passing
- `go build ./...` passes
- `go vet ./...` passes

## Review Notes

- complete
