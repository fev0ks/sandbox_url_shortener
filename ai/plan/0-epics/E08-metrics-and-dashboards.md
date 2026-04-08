# E08 - Metrics And Dashboards

## Summary

Observability epic for turning runtime behavior into low-cardinality technical and business metrics.

## Goal

Instrument the service with Prometheus metrics and provide Grafana dashboards for latency, errors,
cache behavior, and event pipeline health.

## Scope

- Prometheus and Grafana in local stack
- technical metrics for HTTP, database, Redis, and Kafka paths
- business metrics for link creation and redirect outcomes
- dashboards for latency, errors, cache hit ratio, and queue flow
- excludes full alerting and paging policy design

## Key Decisions

- protect label cardinality from the start
- instrument hot paths before adding less critical dimensions
- dashboards should map to known product and operational questions

## Task Links

- `E08-001-add-prometheus-and-grafana-stack.md`
- `E08-002-instrument-http-db-redis-and-kafka-metrics.md`
- `E08-003-add-business-metrics-for-link-and-redirect-flows.md`
- `E08-004-build-mvp-grafana-dashboards.md`

## Open Questions

- whether metrics should be emitted directly from middleware or wrapped in platform helpers
- which dashboard panels are required before the deploy epic can start
