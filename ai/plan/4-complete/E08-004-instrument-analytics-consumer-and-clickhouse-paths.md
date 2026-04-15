# E08-004 - Instrument Analytics Consumer And ClickHouse Paths

Status: complete

Epic: E08 - Metrics And Dashboards

## Summary

Instrument the `analytics` service so Kafka consume, DLQ handling, and ClickHouse write/query paths
show up in metrics and traces.

## Scope

- consumer loop metrics for fetch/process/DLQ outcomes
- spans around consume, decode, sink write, DLQ publish, and stats query paths
- ClickHouse write/query duration and failure metrics
- excludes Grafana dashboard content

## Acceptance Criteria

- `analytics` exposes Prometheus metrics for consumer health and sink outcomes
- Kafka consume and ClickHouse write/query paths produce traces in Tempo
- invalid payload, fetch failure, sink failure, and DLQ publish paths are visible in metrics/traces
- metrics avoid unbounded labels from `event_id`, `link_id`, or payload content

## Assumptions

- consumer loop is the correct boundary for spans and counters, not the Kafka repository internals alone
- one service-level tracing helper is enough for consumer jobs and stats HTTP requests

## Implementation Notes

- keep outcome labels coarse: `result=success|failure`, `reason=invalid_payload|fetch_failed|sink_failed|dlq_failed`
- traces should help answer “where did the event stall” without logging payload contents
- stats query spans should be keyed by operation, not by link id

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add consumer outcome metrics
- [x] add consumer and ClickHouse spans
- [x] add stats query metrics where useful
- [x] update docs and validation notes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./...` for touched packages
- local smoke: malformed payload, DLQ path, and ClickHouse write/query flows change metrics and emit traces

## Review Notes

- `analytics` now exports consumer and ClickHouse metrics with coarse `phase/result` and `operation/result` labels
- live smoke showed `sandbox_url_shortener_analytics_consumer_events_total` increasing on processed redirect events
- ClickHouse raw row count and stats API both matched the redirect traffic generated during smoke
- Kafka trace extraction now preserves the parent runtime context, so sink and DLQ work inherit
  shutdown cancellation while keeping propagated trace headers


## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] Kafka trace extraction now preserves the consumer loop context, so sink and DLQ work do not
  drop shutdown cancellation or deadlines.

### P3 Low

- [x] no open P3 findings
