# E08-003 - Instrument Shortener HTTP Redirect And Create Paths

Status: complete

Epic: E08 - Metrics And Dashboards

## Summary

Instrument `shortener` with the first useful metrics and spans for HTTP request handling, link
creation, redirect resolution, cache behavior, and Kafka publish degradation.

## Scope

- HTTP request duration/error metrics for `shortener`
- spans for create and redirect flows
- cache hit/miss/degrade metrics for redirect resolution
- Kafka publish success/failure metrics and trace events for `link.visited`
- excludes analytics-side consumer instrumentation

## Acceptance Criteria

- `shortener` emits Prometheus metrics for request count/latency and key redirect outcomes
- create and redirect requests produce searchable traces in Tempo
- publish degradation is visible in both metrics and traces
- no high-cardinality labels are introduced from `short_code`, `link_id`, or request ids

## Assumptions

- current handler/service boundaries are good enough to place spans and counters without major refactor
- HTTP middleware can own transport metrics while service code owns domain counters/events

## Implementation Notes

- avoid labels derived from URLs, ids, or user input
- prefer outcome/reason enums such as `status_class`, `result`, `cache_result`, `publish_result`
- use spans to connect create/redirect HTTP requests to downstream service work

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add HTTP metrics and route-safe labels
- [x] add redirect/create spans
- [x] add cache and publish counters
- [x] update docs and validation notes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./...` for touched packages
- local smoke: metrics increase after create/redirect traffic
- local smoke: traces for create/redirect flows are visible in Tempo/Grafana

## Review Notes

- `shortener` now exports HTTP, create, redirect, cache, and publish metrics with bounded labels
- live smoke showed `sandbox_url_shortener_domain_redirect_resolves_total` and `sandbox_url_shortener_eventing_link_visit_publish_total` increasing after redirect traffic
- Tempo trace lookup shows route-aware span names such as `POST /api/v1/links`
- Prometheus validation confirms `/health/*` and `/metrics` no longer appear in shortener HTTP
  request totals


## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] HTTP request metrics now skip `/metrics` and `/health/*`, so request-rate and latency series
  reflect application traffic instead of observability self-scrapes.

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
