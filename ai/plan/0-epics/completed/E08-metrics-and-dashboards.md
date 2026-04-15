# E08 - Metrics And Dashboards

Status: complete

## Summary

Observability epic for adding metrics, traces, and Grafana visibility over the runtime behavior of
the shortener stack.

## Goal

Instrument the services with Prometheus metrics and OpenTelemetry traces, export traces to Tempo,
and provide Grafana dashboards for latency, errors, cache behavior, and event pipeline health.

## Scope

- Prometheus and Grafana in local stack
- Tempo and OTLP plumbing in local stack
- technical metrics for HTTP, database, Redis, and Kafka paths
- business metrics for link creation and redirect outcomes
- traces for HTTP requests, redirect flow, Kafka publish, Kafka consume, and ClickHouse writes
- dashboards for latency, errors, cache hit ratio, queue flow, and trace entry points
- excludes full alerting and paging policy design
- excludes duplicating logs into Grafana/Loki for now

## Key Decisions

- protect label cardinality from the start
- instrument hot paths before adding less critical dimensions
- use OpenTelemetry for tracing instead of service-local tracing helpers
- keep Grafana as the UI for metrics and traces; logs remain in Elasticsearch and Kibana for now
- dashboards should map to known product and operational questions

## Task Links

- `E08-001-add-prometheus-and-grafana-stack.md`
- `E08-002-add-opentelemetry-and-tempo-stack.md`
- `E08-003-instrument-shortener-http-redirect-and-create-paths.md`
- `E08-004-instrument-analytics-consumer-and-clickhouse-paths.md`
- `E08-005-build-mvp-grafana-dashboards-and-trace-workflows.md`

## Open Questions

- how much auto-instrumentation is worth using versus explicit application spans
- which dashboard panels are required before the deploy epic can start
- whether Prometheus should scrape only app `/metrics` endpoints or also observability components
