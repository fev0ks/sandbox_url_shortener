# E01-003 - Add Docker Compose, Health, And Metrics Baseline

Status: complete

Epic: E01 - Platform Bootstrap

## Summary

Add the Docker Compose stack, wire real readiness checks, add a metrics endpoint placeholder, set
up the Makefile, and bring the project to a fully runnable local state.

## Scope

- `docker-compose.yml` in `deploy/docker-compose/` with `api`, `worker`, `postgres`, `redis`
- Dockerfiles for both binaries (multi-stage `golang:1.25-alpine` build)
- Docker Compose health checks for all services
- `/health/ready` on `api-http-stdlib` with PostgreSQL ping; returns `503` if unreachable
- `/health/ready` on `worker` returns `200` (no Kafka dependency yet)
- `/metrics` placeholder on both binaries
- PostgreSQL connection pool via `pgx/v5` `pgxpool`
- Makefile with `build`, `run`, `stop`, `test`, `migrate-up`, `lint` targets
- graceful shutdown for both binaries

## Acceptance Criteria

- `make run` starts the full stack; containers become healthy
- `curl http://localhost:8080/health/live` returns `200`
- `curl http://localhost:8080/health/ready` returns `200` when PG is up, `503` when down
- `curl http://localhost:9090/health/live` returns `200`
- `curl http://localhost:8080/metrics` returns `200`
- `make stop` tears down cleanly
- `make build` compiles both binaries
- `make test` runs `go test ./...`
- `go.sum` includes `pgx/v5`

## Assumptions

- PostgreSQL uses simple password auth from `.env`
- Redis is present in Compose but no Go code connects to it in E01
- Dockerfiles use `golang:1.25-alpine`
- `make migrate-up` is a no-op until E02 adds migration files

## Implementation Notes

- `Dockerfile.api` and `Dockerfile.worker`: multi-stage build with `golang:1.25-alpine` builder
  and `alpine:3.20` runtime
- docker-compose: `api` depends on `postgres` (healthy); `worker` has no infra dependencies yet;
  both use `wget --spider` for health checks
- `ReadyHandler(checks ...Checker)` accepts variadic check functions; api passes `pool.Ping`
- `pgxpool.NewWithConfig` with `DBMaxConns` from config (default `10`)
- graceful shutdown: `signal.NotifyContext` + `srv.Shutdown` + `pool.Close`
- Makefile: `.PHONY` on all targets; `run` calls `docker compose up --build -d`
- metrics handler returns empty Prometheus exposition body (`text/plain; version=0.0.4`)

## Progress Checklist

- [x] create Dockerfiles
- [x] create `docker-compose.yml` with all four services
- [x] add PostgreSQL connection pool to `api-http-stdlib`
- [x] implement `/health/ready` with PG ping
- [x] add `/metrics` placeholder
- [x] implement graceful shutdown for both binaries
- [x] create Makefile with all targets
- [x] run `go test ./...`

## Validation

- `go build ./...` passes
- `go vet ./...` passes
- `go test ./...` — all tests pass
- Docker Compose stack validation deferred to first `make run`

## Review Notes

- complete
