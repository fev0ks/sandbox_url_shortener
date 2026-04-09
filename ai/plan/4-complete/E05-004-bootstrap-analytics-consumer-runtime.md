# E05-004 - Bootstrap Analytics Consumer Runtime

Status: complete

Epic: E05 - Event Pipeline

## Summary

Turn `services/analytics` from a health-only runtime into a real Kafka consumer process that can
read `link.visited` events, validate them, and hand them to a sink boundary without dragging
ClickHouse into the same task.

## Scope

- consumer loop in `services/analytics`
- broker connectivity and consumer group wiring
- deserialize and validate `link.visited` events
- sink boundary for later analytics storage
- readiness and shutdown behavior for the consumer runtime
- excludes ClickHouse persistence

## Acceptance Criteria

- `analytics` can subscribe to the `link.visited` topic and process events
- consumer uses the shared event contract instead of its own shadow structs
- malformed messages are handled explicitly and do not crash the runtime
- readiness reflects the consumer runtime meaningfully once broker dependency exists
- graceful shutdown drains the consumer loop without hanging process termination

## Assumptions

- a log-only or no-op sink is enough before E06 introduces ClickHouse writes
- at-least-once delivery is the baseline; exactly-once semantics are out of scope
- consumer health should focus on broker connectivity and loop liveness, not future storage readiness

## Implementation Notes

- keep runtime ownership inside `services/analytics/internal/app`
- introduce a sink interface so E06 can add ClickHouse without rewriting consumer wiring
- validate and deserialize as close to the broker boundary as possible

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [ ] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./services/analytics/...`
- `go build ./services/analytics/cmd/analytics`
- local smoke:
  - run `go run ./services/analytics/cmd/analytics` with `KAFKA_CONSUMER_ENABLED=true`
  - `curl http://localhost:19090/health/ready`
  - publish a valid `link.visited` JSON payload into Kafka
  - confirm `analytics link.visited consumed` log is emitted
- shutdown smoke:
  - interrupt analytics process
  - confirm `shutdown signal received` is logged and process exits

## Review Notes

- `analytics` now owns a real Kafka consumer loop instead of a health-only runtime
- decode and validation happen at the Kafka boundary via the shared `eventing.LinkVisited` contract
- the sink boundary is intentionally a logging sink in `E05`; ClickHouse wiring is deferred to `E06`
- malformed events are handled explicitly and committed so they do not crash the consumer loop
- broader sink/consumer-path tests are intentionally deferred until post-review hardening

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
