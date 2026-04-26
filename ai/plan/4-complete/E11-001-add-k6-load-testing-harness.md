# E11-001 - Add k6 Load Testing Harness

Status: review

Epic: E11 - Load And Stress Testing

## Summary

Create the first local `k6` harness for exercising the canonical shortener runtime with safe,
repeatable smoke and baseline load runs against create and redirect paths.

## Scope

- add a service-owned load testing tree under `load/k6`
- add shared k6 helpers for configuration, HTTP calls, link creation, and redirect checks
- add initial scenarios for smoke, steady redirect load, and mixed create/redirect load
- add `Taskfile.yaml` entrypoints for running the scenarios locally
- document local prerequisites and safe defaults
- exclude Prometheus/k6 metrics export, exporters, and Grafana dashboards
- exclude saturation and destructive stress scenarios

## Acceptance Criteria

- `load/k6` exists with clear script organization and no dependency on chat history
- a smoke scenario can create a link and verify the redirect endpoint without following traffic to
  public third-party infrastructure
- a steady redirect scenario can reuse a seeded pool of links and hit `GET /{shortCode}`
- a mixed scenario can exercise `POST /api/v1/links` and redirect traffic with conservative
  defaults
- task commands expose the harness through names such as `task load:smoke` and `task load:steady`
- default VU, duration, and request-rate settings are safe for a local laptop

## Assumptions

- `E10` remains skipped; the target runtime is the stdlib shortener service at
  `services/shortener/cmd/shortener`
- load tests target local or controlled environments only
- redirect requests should disable following redirects or use local-only target URLs so tests do
  not generate traffic against public domains
- create-heavy scenarios must account for the existing per-IP create rate limiter

## Implementation Notes

- prefer a Docker/Compose-backed k6 runner if it keeps local setup simpler than requiring a global
  k6 install
- keep script configuration environment-driven: `BASE_URL`, `VUS`, `DURATION`, `LINK_POOL_SIZE`,
  and optional request-rate controls
- avoid high-cardinality tags such as `short_code`, `link_id`, request id, or original URL
- make the harness useful before metrics export exists: console summary and thresholds should
  already identify obvious failures
- model redirects as API latency checks, not as end-to-end public web navigation

## Progress Checklist

- [x] confirm scope and assumptions
- [x] choose local k6 execution shape
- [x] add `load/k6` structure and shared helpers
- [x] add smoke and baseline scenarios
- [x] add Taskfile load entrypoints
- [x] document local usage and safety defaults
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task --list` passes and shows load tasks
- `node --check load/k6/lib/*.js load/k6/scenarios/*.js` passes
- `task test` passes
- `task deploy` attempted, but runtime validation is blocked by Docker pull hanging for new exporter images
- still needs `task load:smoke` and `task load:steady` after Docker images are available

## Review Notes

- resolved review feedback for bounded k6 labels and limiter-safe seed defaults; runtime validation pending image availability

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] Default load scenarios trip the create limiter
  - `LINK_POOL_SIZE` defaults to `20`, while the shortener create limiter defaults to `10`
    requests/minute per client IP.
  - `seedLinks()` creates one fresh link per pool item from the same k6 container before redirect
    load starts, so `load:steady`, `load:spike`, `load:saturation`, and `load:mixed` can fail in
    setup with 429s before testing the redirect path.
  - Lower the default pool size, raise `RATE_LIMIT_PER_MIN` for k6 runs, or seed through a
    controlled fixture path that does not consume the public create limiter.
  - Resolution: default `LINK_POOL_SIZE` is now `5`, create traffic is paced, and create requests
    use a bounded TEST-NET-2 `X-Forwarded-For` pool for local multi-client modeling.
  - Source: `load/k6/lib/config.js:31`

### P2 Medium

- [x] Redirect metrics still carry dynamic short-code URLs
  - Redirect requests use `/${shortCode}` and only add an `endpoint` tag.
  - k6 emits URL/name tags by default, so the Prometheus remote-write series will still include
    per-short-code paths, violating the E11 requirement that k6 labels not include `short_code` or
    unbounded URLs.
  - Add stable request naming or URL grouping, and consider trimming `systemTags` so the raw `url`
    tag does not persist dynamic paths.
  - Resolution: k6 scenarios now trim `systemTags` to exclude `url` and override request names
    with bounded values such as `GET /:shortCode`.
  - Source: `load/k6/lib/http.js:38`

### P3 Low

- [x] no open P3 findings
