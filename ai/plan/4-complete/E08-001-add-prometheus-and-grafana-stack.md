# E08-001 - Add Prometheus And Grafana Stack

Status: complete

Epic: E08 - Metrics And Dashboards

## Summary

Add `Prometheus` and `Grafana` to the local stack and wire the current app `/metrics` endpoints into
Prometheus scraping.

## Scope

- Docker Compose services for Prometheus and Grafana
- Prometheus scrape config for `shortener` and `analytics`
- local persistence/config for Grafana dashboards provisioning if needed later
- excludes Tempo and tracing export
- excludes dashboard content beyond proving the stack is reachable

## Acceptance Criteria

- local Compose starts `prometheus` and `grafana` alongside the current app stack
- Prometheus successfully scrapes `shortener` and `analytics` `/metrics`
- Grafana is reachable locally and can query Prometheus
- docs reflect the local metrics stack and access points

## Assumptions

- current `/metrics` endpoints are already stable enough to become Prometheus scrape targets
- local file-based provisioning is sufficient before any Helm or production deploy work

## Implementation Notes

- keep orchestration in `deploy/docker-compose`
- prefer explicit scrape jobs over dynamic discovery
- keep scrape labels low-cardinality and readable

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add Prometheus and Grafana services to local stack
- [x] add Prometheus scrape config for app services
- [x] update docs and access notes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- local smoke: Prometheus targets are up and Grafana can query Prometheus

## Review Notes

- `prometheus`, `grafana`, `shortener`, and `analytics` are healthy in local Compose
- Prometheus `/api/v1/targets` shows `shortener`, `analytics`, and `prometheus` as `up`
- Grafana datasource provisioning loads `Prometheus` and `Tempo`


## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
