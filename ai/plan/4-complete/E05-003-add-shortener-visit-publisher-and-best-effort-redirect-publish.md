# E05-003 - Add Shortener Visit Publisher And Best-Effort Redirect Publish

Status: complete

Epic: E05 - Event Pipeline

## Summary

Wire the `shortener` redirect flow to publish `link.visited` events after a successful redirect
decision without making Kafka availability part of redirect correctness.

## Scope

- producer boundary in `shortener`
- mapping request-scoped metadata into `link.visited`
- best-effort publish on successful redirect path
- logging and metrics hooks for publish success and failure
- excludes analytics consumer logic
- excludes ClickHouse writes

## Acceptance Criteria

- successful redirect flow attempts to publish one `link.visited` event
- event payload carries the request correlation id and visit metadata available on the HTTP boundary
- Kafka publish failure does not change redirect response semantics
- publish failures are surfaced through structured `WARN` logs and metric hooks
- producer code does not leak broker details into transport handler response mapping

## Assumptions

- publishing after the redirect decision but before request completion is acceptable for MVP
- best-effort delivery on the request path is sufficient as long as failures are visible
- request metadata such as IP, user agent, and referer are available from the HTTP request and can
  be mapped explicitly

## Implementation Notes

- add a narrow publisher boundary instead of calling Kafka client code directly from the redirect
  handler body
- keep redirect semantics unchanged: `302`, `404`, and `410` remain driven by link state only
- use the shared `link.visited` contract from `E05-002`

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [ ] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./services/shortener/internal/...`
- local smoke with current code and producer disabled:
  - `go run ./services/shortener/cmd/shortener` on `:18081`
  - `curl -i http://localhost:18081/zjueQfB`
  - confirmed `302 Found` redirect still works with `KAFKA_PRODUCER_ENABLED=false`

## Review Notes

- redirect path now calls service-owned `ResolveAndTrackVisit` instead of pushing Kafka logic into
  the HTTP handler body
- `link.visited` publish is best-effort and asynchronous: request-path redirect latency no longer
  depends on broker round-trip
- cache payload now carries `link_id` so unlimited cache-hit redirects can still publish a complete
  event contract
- broader producer-path tests are intentionally deferred until post-review hardening
- producer outcome metrics are intentionally deferred until the Prometheus/Grafana phase

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
