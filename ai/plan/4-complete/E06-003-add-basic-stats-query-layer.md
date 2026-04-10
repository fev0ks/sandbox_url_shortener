# E06-003 - Add Basic Stats Query Layer

Status: complete

Epic: E06 - Analytics And Stats

## Summary

Add a ClickHouse-backed stats read model in `analytics` for total clicks, time buckets, and top referrers.

## Scope

- ClickHouse read adapter in `analytics`
- service-layer stats query orchestration
- baseline aggregates from raw events
- excludes UI wiring

## Acceptance Criteria

- `analytics` can query basic stats for a link from ClickHouse
- query layer reads from raw events and returns a stable API-friendly model
- aggregates include total clicks, time buckets, and top referrers
- stats path keeps ClickHouse ownership inside `analytics`

## Assumptions

- querying raw events directly is acceptable at MVP scale
- stats can key off `link_id` to fit the current details-page flow

## Implementation Notes

- keep store-specific SQL in repository layer
- service layer should orchestrate stats fetch from raw analytics events
- prefer a simple fixed bucket window first, such as recent hourly buckets

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go build ./services/analytics/cmd/analytics`
- `go test ./services/analytics/internal/service ./services/analytics/internal/transport/http/stdlib/stats`
- local smoke: query stats for a link with ingested events

## Review Notes

- Stats are keyed by `link_id` to match the existing details-page flow and to avoid introducing a second public identity for analytics reads.
- `analytics` owns the ClickHouse read adapter and returns total clicks, recent hourly buckets, and top referrers over its own HTTP surface.
- Unit tests now cover the stats service wrapper and HTTP handler response contract.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
