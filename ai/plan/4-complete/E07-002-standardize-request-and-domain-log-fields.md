# E07-002 - Standardize Request And Domain Log Fields

Status: complete

Epic: E07 - Log Observability

## Summary

Normalize the structured log field set across `shortener` and `analytics` so request and event investigations use consistent searchable fields.

## Scope

- request and domain log field conventions for both services
- log call cleanup where important fields are missing or inconsistently named
- shared guidance updates in checked-in docs
- excludes Elasticsearch dashboards, metrics, or alerting

## Acceptance Criteria

- request and event logs consistently use fields such as `request_id`, `short_code`, `link_id`, `status_code`, `operation`, and `error_kind` where applicable
- create, redirect, Kafka publish/degradation, consumer failure, and ClickHouse write paths emit searchable structured fields
- logs do not add sensitive payloads or secrets while improving debugging value
- docs reflect the chosen field conventions and search expectations

## Assumptions

- not every log line needs every field; the goal is consistent key naming on the important paths
- existing `slog` JSON output remains the application boundary format

## Implementation Notes

- keep field naming stable across services rather than inventing service-local variants
- prefer adding context-rich fields to important lifecycle logs over increasing log volume everywhere
- if a field is unavailable on a path, omit it rather than emitting misleading placeholders

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./internal/platform/middleware ./services/shortener/internal/transport/http/stdlib/link ./services/shortener/internal/transport/http/stdlib/redirect ./services/shortener/internal/service ./services/analytics/internal/app ./services/analytics/internal/service`
- `go test ./internal/platform/middleware ./services/shortener/internal/app`
- local smoke still pending for Kibana-side searchability; this task only normalized app field names in code and docs

## Review Notes

- access logs now emit stable `operation=http_request`, normalized `client_ip`, and `user_agent`
- `shortener` create/get/list/delete/enable/disable/redirect paths now use stable `operation` values and use `error_kind` on degraded or error outcomes
- `shortener` service warnings for cache and event publish paths now use stable `operation` and `error_kind`
- `analytics` consumer and sink logs now use stable `operation` values for fetch/process/DLQ/write paths
- unit tests now cover the access log field contract and async publish failure log fields
- docs now describe the shared searchable field set explicitly

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] `user_agent` was added to access logs, and the docs were narrowed to the current stdlib
  router reality by standardizing `path` rather than promising a separate `route` field that is
  not available without extra route-pattern propagation plumbing.

### P3 Low

- [x] no open P3 findings
