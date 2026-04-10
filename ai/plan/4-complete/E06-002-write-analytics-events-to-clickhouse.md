# E06-002 - Write Analytics Events To ClickHouse

Status: complete

Epic: E06 - Analytics And Stats

## Summary

Replace the bootstrap logging sink in `analytics` with a real ClickHouse write sink for raw click events.

## Scope

- ClickHouse write adapter in `analytics`
- sink wiring from Kafka consumer to raw events table
- idempotency-aware raw write behavior suitable for at-least-once delivery
- excludes stats read API and UI

## Acceptance Criteria

- valid `link.visited` events are persisted into `click_events_raw`
- analytics startup fails clearly when ClickHouse config is invalid or unreachable
- ingestion keeps the raw append-oriented event shape
- docs and task notes explain the current dedupe strategy

## Assumptions

- storing raw events with query-time dedupe is acceptable for MVP scale
- exact dedupe optimization can stay deferred as long as stats remain correct

## Implementation Notes

- keep writes in repository layer
- preserve `event_id` so later reads can dedupe duplicates
- avoid adding aggregation logic to the ingest path

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go build ./services/analytics/cmd/analytics`
- `go test ./services/analytics/internal/repository/clickhouse`
- local smoke: publish `link.visited` and confirm row exists in ClickHouse

## Review Notes

- `analytics` no longer uses the bootstrap logging sink in the main path; the consumer now writes raw events through a ClickHouse repository adapter.
- Dedupe stays query-time via `event_id` and `uniqExact(...)`; raw ingestion remains append-oriented for now.
- Unit tests now cover sink input validation, exec error wrapping, and the stats repository query contract without needing a live ClickHouse instance.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] Buffered or batched writes remain a follow-up optimization. The current one-row insert path is
  accepted for MVP scale and is covered by repository tests plus live smoke validation.

### P3 Low

- [x] no open P3 findings
