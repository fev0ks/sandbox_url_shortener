# E11 - Load And Stress Testing

## Summary

Performance engineering epic for exercising the service under sustained and spiky parallel traffic
while observing application and infrastructure behavior in real time.

## Goal

Add repeatable load and stress scenarios that hit `POST` and `GET` paths with high concurrency and
visualize the resulting application, database, cache, and test-driver metrics in Grafana.

## Scope

- `k6`-based load testing scenarios
- real-time k6 metrics export to Prometheus or a compatible metrics pipeline
- Grafana dashboards that combine test-driver and application metrics
- PostgreSQL and Redis exporter metrics for infrastructure visibility
- scenarios such as steady redirect load, mixed read/write load, spike load, and saturation testing
- excludes running abusive traffic against third-party or public infrastructure

## Key Decisions

- load testing targets only local or controlled environments owned by the project
- the baseline scenario set must include both heavy `GET` redirect traffic and parallel `POST`
  traffic
- application metrics and test-driver metrics must be visible on the same dashboard during a run
- this epic starts only after the core metrics stack is available

## Task Links

- `E11-001-add-k6-load-testing-harness.md`
- `E11-002-export-k6-metrics-to-prometheus.md`
- `E11-003-add-postgres-and-redis-exporters.md`
- `E11-004-build-load-testing-dashboards.md`
- `E11-005-create-steady-spike-and-saturation-scenarios.md`

## Open Questions

- whether Prometheus remote write is enough for k6, or a different sink is worth evaluating later
- what safe default limits should be used to avoid overwhelming a local laptop by accident
