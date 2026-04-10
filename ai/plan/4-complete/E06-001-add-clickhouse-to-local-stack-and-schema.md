# E06-001 - Add ClickHouse To Local Stack And Schema

Status: complete

Epic: E06 - Analytics And Stats

## Summary

Add ClickHouse to the local runtime and create the initial raw events schema for analytics writes.

## Scope

- ClickHouse service in Docker Compose
- service-owned schema files for analytics raw events
- runtime config for `analytics` with shared connection helper support
- excludes analytics write path and stats queries

## Acceptance Criteria

- local Docker Compose can start ClickHouse
- initial `click_events_raw` schema is created automatically in local runtime
- `analytics` has explicit ClickHouse config and shared connection setup is available for analytics-owned adapters
- docs reflect ClickHouse runtime shape and schema ownership

## Assumptions

- local schema bootstrap can use container init SQL for now
- one raw events table is enough before materialized views

## Implementation Notes

- keep ClickHouse DDL under `services/analytics/migrations/clickhouse`
- use a simple local database name such as `analytics`
- keep config service-owned even if connection helper becomes shared

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- `go build ./services/analytics/cmd/analytics ./services/shortener/cmd/shortener`
- local smoke: ClickHouse starts and `click_events_raw` exists

## Review Notes

- `ClickHouse` now lives in the local compose stack and schema bootstrap is owned by `services/analytics/migrations/clickhouse/init`.
- Runtime config stays explicit in `analytics`, while shared connection setup lives in `internal/platform/clickhouse` for analytics-owned adapters.
- This slice is mostly runtime/config ownership, so validation stays at build and compose-smoke level rather than dedicated unit tests.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] Configurable local database bootstrap remains a follow-up. The MVP local stack intentionally
  standardizes on `analytics` for now, so the hard-coded init SQL is accepted until multi-database
  local setup becomes a real requirement.

### P3 Low

- [x] no open P3 findings
