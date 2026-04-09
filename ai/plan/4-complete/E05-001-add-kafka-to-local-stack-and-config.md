# E05-001 - Add Kafka To Local Stack And Config

Status: complete

Epic: E05 - Event Pipeline

## Summary

Introduce Kafka into the local runtime and extend `shortener` and `analytics` config so both
services can be wired against a broker explicitly instead of hiding transport settings in ad-hoc
environment variables later.

## Scope

- Kafka and required local companion services in Docker Compose
- local topic naming defaults for main event flow and DLQ
- runtime config for broker addresses, topic names, consumer group, and producer toggle
- readiness implications for `analytics`
- excludes producer business logic
- excludes ClickHouse ingestion

## Acceptance Criteria

- local Docker Compose can start Kafka as part of the project stack
- `shortener` and `analytics` have explicit validated config for Kafka connectivity
- topic names for main flow and DLQ are defined in one place and do not rely on string literals
- `analytics` config and readiness rules are prepared for a broker dependency
- repo docs and epic notes reflect the local Kafka runtime shape

## Assumptions

- Docker-first local runtime remains the default for infra dependencies
- Kafka topic bootstrap can stay local-only and does not need a production topic-management story yet
- `analytics` is allowed to remain functionally narrow until the consumer loop lands in later E05 tasks

## Implementation Notes

- keep broker config service-owned: `services/shortener/internal/config` and
  `services/analytics/internal/config`
- define topic names centrally so later producer and consumer code reuse the same constants
- prefer a local stack that is easy to inspect and reset over a production-like multi-broker setup

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add Kafka to local runtime and config
- [ ] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./services/shortener/internal/config ./services/analytics/internal/config`
- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- `docker compose -f deploy/docker-compose/docker-compose.yml up -d kafka kafka-init`
- `docker exec docker-compose-kafka-1 kafka-topics --bootstrap-server localhost:9092 --list`
  confirmed:
  - `link.visited.v1`
  - `link.visited.v1.dlq`

## Review Notes

- local stack now includes single-node Kafka with internal listener `kafka:9092` and host listener
  `localhost:29092`
- shared event-pipeline defaults live in `internal/platform/eventing/topics.go`
- service-owned config now carries broker, topic, toggle, and consumer-group fields
- broader E05 tests are intentionally deferred until post-review hardening; this task only keeps
  lightweight config validation and local runtime proof

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
