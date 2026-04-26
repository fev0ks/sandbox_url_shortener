# Load Testing Workflows

This document describes the local E11 performance loop for k6, Prometheus, Grafana, PostgreSQL,
and Redis visibility.

## Local Commands

Start the stack first:

```bash
task deploy
```

Run safe baseline scenarios:

```bash
task load:smoke
task load:steady
task load:mixed
```

Run scenarios with k6 metrics exported to Prometheus:

```bash
task load:steady:metrics
task load:mixed:metrics
task load:spike:metrics
```

Run guarded saturation only when you explicitly want to stress local resources:

```bash
ALLOW_SATURATION=1 task load:saturation
```

## Configuration

The k6 scripts read these environment variables:

- `BASE_URL` defaults to `http://shortener:8080` inside Docker Compose
- `VUS` defaults to `5`
- `DURATION` defaults to `30s`
- `LINK_POOL_SIZE` defaults to `5`
- `CREATE_WEIGHT` defaults to `0.02` for mixed traffic
- `REQUEST_DELAY_SECONDS` defaults to `1`
- `CREATE_CLIENT_IP_POOL` defaults to `20`
- `SPIKE_PEAK_VUS` defaults to `20`
- `SATURATION_VUS` defaults to `50`
- `SATURATION_DURATION` defaults to `2m`

Defaults are intentionally small so the scenarios are safe on a local laptop. Create requests use
a bounded pool of TEST-NET-2 `X-Forwarded-For` addresses so the local Docker runner can model a
small client set without tripping the default per-IP create limiter during seed setup.

## Scenario Intent

- `load:smoke` creates one link and verifies redirect behavior without following the redirect to
  the target URL.
- `load:steady` seeds a pool of links and sends steady redirect traffic.
- `load:mixed` combines controlled link creation with redirect traffic.
- `load:spike` ramps redirect traffic up and down quickly.
- `load:saturation` is guarded by `ALLOW_SATURATION=1` and is not a normal quality gate.

Redirect checks use local health URLs and disable following redirects. The tests should not send
traffic to public third-party infrastructure.

## k6 Metrics Export

The local stack uses k6 Prometheus remote write output:

```bash
k6 run -o experimental-prometheus-rw script.js
```

Prometheus is started with `--web.enable-remote-write-receiver`, and k6 writes to:

```text
http://prometheus:9090/api/v1/write
```

k6 metric series use the `k6_` prefix. Useful starter queries:

```promql
sum(rate(k6_http_reqs_total[1m]))
k6_http_req_duration_milliseconds_p95
k6_http_req_duration_milliseconds_p99
k6_http_req_failed_rate
```

The k6 output is still marked experimental by Grafana k6, so keep the integration small and
document any version-specific changes if it moves.

## Infrastructure Metrics

Prometheus also scrapes:

- `postgres-exporter:9187`
- `redis-exporter:9121`

Useful signals during load runs:

- PostgreSQL connection pressure and transaction/query activity
- Redis command rate, memory usage, and evictions
- shortener HTTP latency and error ratio
- redirect cache outcomes
- Kafka publish outcomes

## Grafana

Open Grafana at:

```text
http://localhost:3001
```

Use the `Load Testing Overview` dashboard during a run. Read it in this order:

1. k6 request rate and latency
2. shortener HTTP latency and errors
3. redirect cache outcomes
4. PostgreSQL and Redis exporter panels
5. Kafka publish and analytics pipeline outcomes

Keep labels bounded. The scripts override k6 request names and disable the default `url` system tag
so short codes and full URLs do not become Prometheus labels.
