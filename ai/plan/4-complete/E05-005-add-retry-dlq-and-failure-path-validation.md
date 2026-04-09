# E05-005 - Add Retry DLQ And Failure Path Validation

Status: complete

Epic: E05 - Event Pipeline

## Summary

Add the first operational safety net around the event pipeline: explicit retry behavior, DLQ
handling, and validation of poison-message and broker-degradation scenarios.

## Scope

- retry policy for transient consumer failures
- dedicated DLQ topic and event forwarding rules
- failure classification for malformed versus transient processing errors
- local validation of degraded producer and consumer paths
- excludes ClickHouse-specific retry tuning

## Acceptance Criteria

- the event pipeline has an explicit retry policy instead of implicit infinite loops
- poison or permanently invalid messages are routed to a DLQ topic
- transient failures are retried in a bounded and observable way
- local validation covers at least malformed-event handling and downstream-processing failure behavior
- docs and task notes explain what remains deferred to E06 or later

## Assumptions

- exact backoff sophistication can stay simple in MVP as long as it is bounded and visible
- a dedicated DLQ topic is worth materializing now to keep main-topic consumption healthy
- local failure-path testing is sufficient before adding heavier observability in E07 and E08

## Implementation Notes

- avoid hiding retry behavior inside Kafka client defaults alone; keep project-level semantics explicit
- classify failures into at least schema or decode failure versus sink or downstream failure
- capture enough logging context to debug why an event was retried or dead-lettered

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./services/analytics/...`
- `go build ./services/analytics/cmd/analytics`
- local smoke: inject malformed event into main topic and confirm payload appears in `link.visited.v1.dlq`
- unit validation: failing sink path retries three times, then publishes to DLQ and commits the source message

## Review Notes

- `analytics` now has explicit bounded retry semantics instead of implicit infinite retry inside the main loop
- malformed payloads are classified as permanent failures and forwarded to DLQ immediately
- sink failures are treated as transient but bounded: three attempts, then DLQ
- DLQ messages preserve the original payload and add headers with reason, attempts, and source location
- sink-failure behavior is validated with focused unit tests; broader observability and replay tooling remain deferred to later epics

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
