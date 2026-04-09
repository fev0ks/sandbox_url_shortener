# E05 - Event Pipeline

Status: complete

## Summary

Integration epic for introducing asynchronous click event delivery without adding redirect-path
coupling or forcing analytics storage work too early.

## Goal

Publish `link.visited` events from the `shortener` service and consume them in an `analytics`
process through Kafka with operational basics such as retries, DLQ, and correlation fields ready
for later ClickHouse ingestion.

## Scope

- Kafka in local Docker Compose
- runtime config for `shortener` and `analytics`
- versioned `link.visited` event schema
- producer boundary and publish wiring in redirect flow
- `analytics` consumer bootstrap
- retry handling and DLQ strategy
- excludes analytics storage implementation in ClickHouse

## Key Decisions

- redirect success must not depend on synchronous Kafka delivery
- event schema must be versioned from day one
- consumer side should be designed for at-least-once processing
- `shortener` and `analytics` stay separate binaries inside one repository
- shared event contract must live in repo-owned code, not in ad-hoc JSON maps duplicated between
  producer and consumer
- a dedicated DLQ topic should exist in E05 so poison events do not block the main consumer path

## Task Links

- `E05-001-add-kafka-to-local-stack-and-config.md`
- `E05-002-define-link-visited-event-schema.md`
- `E05-003-add-shortener-visit-publisher-and-best-effort-redirect-publish.md`
- `E05-004-bootstrap-analytics-consumer-runtime.md`
- `E05-005-add-retry-dlq-and-failure-path-validation.md`

## Open Questions

- which Go Kafka client best fits the repo's simplicity goals while covering both producer and
  consumer use cases cleanly
- whether topic creation in local dev should stay Docker-init driven or move into a small explicit
  bootstrap command later

## Completion Note

`E05` now provides a working local Kafka event pipeline: `shortener` publishes `link.visited`
asynchronously, `analytics` consumes and validates the shared contract, malformed and terminally
failed messages flow into DLQ, and the local runtime includes broker and topic bootstrap defaults.
