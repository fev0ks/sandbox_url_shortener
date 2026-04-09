# E05-002 - Define Link Visited Event Schema

Status: complete

Epic: E05 - Event Pipeline

## Summary

Define the versioned `link.visited` event contract and put the shared payload, codec, and topic
constants into repo-owned code so producer and consumer evolve against the same schema.

## Scope

- versioned `link.visited` event payload
- shared event package or contract location used by both services
- JSON encoding and decoding helpers or equivalent typed contract usage
- required correlation and visit fields for analytics ingestion
- excludes producer wiring
- excludes consumer loop

## Acceptance Criteria

- `link.visited` has an explicit versioned schema
- payload includes `event_id`, `request_id`, `link_id`, `short_code`, `occurred_at`, `ip`,
  `user_agent`, and `referer`
- producer and consumer can import the same contract definitions
- schema tests cover encode/decode round-trips and missing required field behavior
- the contract does not include premature enrichment such as `country`

## Assumptions

- JSON is acceptable for the first event contract; protobuf or Avro can stay deferred
- `country` enrichment remains an analytics-side concern in E06, not part of the Kafka payload
- the payload should stay append-friendly and browser-request-derived rather than reconstructing state later

## Implementation Notes

- prefer a small shared package under repo-owned code over duplicated structs in `shortener` and
  `analytics`
- encode correlation intent from day one: the same `request_id` should connect HTTP logs, Kafka
  events, and analytics logs
- avoid embedding storage-only or ClickHouse-specific fields into the transport contract

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [ ] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./internal/platform/eventing`
- manual contract check against `docs/system_design.md` and `docs/data_and_logging_strategy.md`

## Review Notes

- shared contract now lives in `internal/platform/eventing`
- `link.visited` carries explicit `event_name` and `version` fields plus request-derived visit
  metadata needed by later producer and consumer work
- broader encode/decode tests are intentionally deferred until post-review hardening

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
