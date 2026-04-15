# E08-005 - Build MVP Grafana Dashboards And Trace Workflows

Status: complete

Epic: E08 - Metrics And Dashboards

## Summary

Create the first operator-facing Grafana dashboards and trace lookup workflows so the metrics and
Tempo plumbing become usable, not just deployed.

## Scope

- Grafana data source wiring for Prometheus and Tempo
- MVP dashboards for HTTP latency/errors, redirects, cache behavior, Kafka publish health, and analytics consumer flow
- starter trace lookup workflow notes for create, redirect, publish failure, and consumer failure
- excludes alerting rules and notification policies

## Acceptance Criteria

- Grafana can query both Prometheus and Tempo locally
- at least one dashboard exists for `shortener` and one for `analytics`
- operator can move from a metric panel to a trace search with the documented workflow
- docs capture the baseline dashboard/tracing usage

## Assumptions

- dashboard JSON provisioning in-repo is acceptable for the MVP
- a small number of high-signal panels is better than broad panel count

## Implementation Notes

- optimize for known questions, not dashboard completeness
- every panel should answer a concrete operational question
- include trace entry points, but do not force log duplication into Grafana

## Progress Checklist

- [x] confirm scope and assumptions
- [x] wire Grafana data sources for Prometheus and Tempo
- [x] add MVP dashboards
- [x] add trace workflow notes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- local smoke: dashboards render live metrics
- local smoke: Grafana can find traces from Tempo for create/redirect/consumer flows

## Review Notes

- Grafana provisions `Prometheus` and `Tempo` automatically
- MVP dashboards `Shortener Overview` and `Analytics Overview` load via provisioning
- `docs/metrics_and_tracing_workflows.md` now documents the local Grafana and Tempo workflow
- `Shortener Overview` now includes `Redirect Cache Outcomes` for cache hit/miss/degrade visibility

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] `Shortener Overview` now includes a `Redirect Cache Outcomes` panel backed by
  `sandbox_url_shortener_domain_redirect_cache_lookups_total`.

### P3 Low

- [x] no open P3 findings
