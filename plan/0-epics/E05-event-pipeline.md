# E05 - Event Pipeline

## Summary

Integration epic for introducing asynchronous click event delivery without adding redirect-path
coupling.

## Goal

Publish `link.visited` events from the `api` service and consume them in a `worker` process through
Kafka with operational basics such as retries and DLQ planning.

## Scope

- Kafka in local Docker Compose
- versioned `link.visited` event schema
- producer wiring in redirect flow
- `worker` consumer bootstrap
- retry handling and DLQ strategy
- excludes analytics storage implementation in ClickHouse

## Key Decisions

- redirect success must not depend on synchronous Kafka delivery
- event schema must be versioned from day one
- consumer side should be designed for at-least-once processing
- `api` and `worker` stay separate binaries inside one repository

## Task Links

- `E05-001-add-kafka-to-local-stack-and-config.md`
- `E05-002-define-link-visited-event-schema.md`
- `E05-003-publish-click-events-from-redirect-flow.md`
- `E05-004-bootstrap-worker-consumer-and-retry-path.md`

## Open Questions

- whether to materialize a dedicated DLQ topic immediately or document it first
- which Kafka client best fits the repo's simplicity goals
