# E01-001 - Establish Repository Layout And API Entrypoint

Status: complete

Epic: E01 - Platform Bootstrap

## Summary

Create the directory structure for the modular monolith and wire the initial `cmd/api-http-stdlib`
and `cmd/worker` entrypoints so both binaries compile and start an HTTP server that responds on
health endpoints.

## Scope

- decide and set `go.mod` module path
- create the package layout as defined in `docs/url_shortener_codex_plan.md` section 7
- wire `cmd/api-http-stdlib/main.go` to start an HTTP server on port `8080`
- wire `cmd/worker/main.go` to start an HTTP server on port `9090` (health/metrics only, no Kafka)
- add placeholder `/health/live` returning `200 OK` on both binaries
- remove the GoLand boilerplate from the existing `main.go`
- verify both binaries compile with `go build ./...`

## Acceptance Criteria

- `go build ./cmd/api-http-stdlib` produces a binary that starts and responds on `/health/live`
- `go build ./cmd/worker` produces a binary that starts and responds on `/health/live`
- directory structure matches the planned layout
- no GoLand template code remains
- `go vet ./...` passes

## Assumptions

- no external dependencies beyond the standard library are introduced in this task
- health endpoints return a plain `200` with a minimal JSON body (`{"status": "ok"}`)
- detailed health check logic (readiness with DB ping) is wired in E01-003

## Implementation Notes

- used `net/http` directly for both servers, no third-party routers
- shared health handler lives in `internal/platform/health/`
- `LiveHandler()` returns `{"status": "ok"}`, `ReadyHandler(checks ...Checker)` accepts optional
  dependency checks (used in E01-003 for PG ping)
- both `main.go` files are thin: parse config, build deps, start server, handle shutdown signal
- old root `main.go` with GoLand boilerplate removed
- kept `go.mod` module path as `sandbox_url_shortener`

## Progress Checklist

- [x] confirm scope and assumptions
- [x] update `go.mod` module path
- [x] create directory structure
- [x] implement `cmd/api-http-stdlib/main.go`
- [x] implement `cmd/worker/main.go`
- [x] implement shared health handler
- [x] remove GoLand boilerplate `main.go`
- [x] run `go build ./...` and `go vet ./...`

## Validation

- `go build ./...` passes
- `go vet ./...` passes

## Review Notes

- complete
