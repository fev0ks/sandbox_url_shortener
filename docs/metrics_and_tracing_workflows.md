# Metrics And Tracing Workflows

This document is the operator quick-start for the E08 observability stack.

## Local Access Points

- Grafana: `http://localhost:3001`
- Prometheus: `http://localhost:9091`
- Tempo HTTP API: `http://localhost:3200`
- shortener metrics: `http://localhost:8080/metrics`
- analytics metrics: `http://localhost:9090/metrics`

Grafana uses the default local credentials unless you changed them manually:

- username: `admin`
- password: `admin`

## Provisioned Data Sources

Grafana is provisioned with:

- `Prometheus` for application and pipeline metrics
- `Tempo` for distributed traces

Two baseline dashboards are provisioned automatically:

- `Shortener Overview`
- `Analytics Overview`
- `Load Testing Overview`

See [Load Testing Workflows](load_testing_workflows.md) for k6 scenarios, Prometheus remote write,
PostgreSQL exporter, and Redis exporter usage.

## What The Stack Covers

### Metrics

The services export low-cardinality Prometheus metrics for:

- HTTP request count and latency by `service`, `method`, `route`, `status_code`
- link create outcomes
- redirect resolve outcomes
- redirect cache outcomes
- async `link.visited` publish outcomes
- analytics consumer outcomes
- ClickHouse operation outcomes and durations

### Traces

The services emit OpenTelemetry traces for:

- incoming HTTP requests
- shortener create flow
- shortener redirect flow
- async `link.visited` publish dispatch
- analytics consumer processing
- DLQ publish
- ClickHouse insert and stats queries

## Daily Checks

### Prometheus Targets

Open [http://localhost:9091/targets](http://localhost:9091/targets).

Expected healthy targets:

- `shortener`
- `analytics`
- `postgres-exporter`
- `redis-exporter`
- `prometheus`

### Grafana Dashboards

Open [http://localhost:3001/dashboards](http://localhost:3001/dashboards).

Use:

- `Shortener Overview` when debugging create, redirect, cache, and publish health
- `Analytics Overview` when debugging consumer, DLQ, and ClickHouse behavior

### Trace Search In Grafana

Open Grafana Explore and select the `Tempo` datasource.

Useful starting filters:

- service name `shortener`
- service name `analytics`

Known flows:

- create request should produce HTTP + service spans in `shortener`
- redirect request should produce HTTP + resolve + publish spans in `shortener`
- consumed Kafka event should produce process + ClickHouse spans in `analytics`

## Validation Loop

Use this sequence when validating the local stack:

1. create a link through `web` or `shortener`
2. perform one or more redirects
3. open `Shortener Overview` and confirm request/publish counters move
4. open `Analytics Overview` and confirm consumer or ClickHouse counters move
5. open Grafana Explore with `Tempo` and search recent traces for `shortener`
6. repeat with an analytics event or malformed Kafka payload when debugging consumer paths

## Guardrails

- do not add labels derived from `short_code`, `link_id`, request ids, or payload contents
- prefer coarse results such as `success`, `failure`, `miss`, `hit`, `dlq`
- use logs in Kibana for payload-adjacent debugging; keep Prometheus labels bounded
