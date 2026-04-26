# E11-005 - Create Steady Spike And Saturation Scenarios

Status: review

Epic: E11 - Load And Stress Testing

## Summary

Expand the baseline k6 harness into a practical scenario set for steady load, mixed traffic, spike
load, and guarded saturation testing while keeping dangerous defaults opt-in.

## Scope

- add steady redirect load scenario with safe local defaults
- add mixed create/redirect scenario with controlled create rate
- add spike scenario that ramps traffic up and down quickly
- add saturation scenario behind an explicit guard such as `ALLOW_SATURATION=1`
- document what each scenario is meant to prove and when not to run it
- exclude third-party/public target testing
- exclude changing application behavior based on findings; performance fixes should become
  follow-up tasks

## Acceptance Criteria

- each scenario has a Taskfile entrypoint and documented default settings
- steady and mixed scenarios are safe to run on a local laptop by default
- spike and saturation scenarios require intentional configuration before they can stress the
  machine aggressively
- scenario scripts reuse the shared harness from `E11-001`
- scenario docs explain expected metrics to watch in the load-testing dashboard
- load results produce enough evidence to open follow-up optimization tasks when bottlenecks are
  found

## Assumptions

- `E11-001` has established the harness structure and shared helpers
- `E11-002` and `E11-004` have established metrics visibility for interpreting runs
- saturation is a learning tool for local controlled environments, not a default quality gate

## Implementation Notes

- prefer scenario names that describe traffic shape, not implementation details
- use local seed data or setup functions so redirect load has a stable pool of short codes
- record thresholds for failure rate and latency, but keep them realistic for local hardware
- make the output actionable: each scenario should point to which dashboard panels validate its
  behavior
- do not add scenario labels with high-cardinality values

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add steady redirect scenario
- [x] add mixed create/redirect scenario
- [x] add spike scenario
- [x] add guarded saturation scenario
- [x] add Taskfile commands and docs for each scenario
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `node --check load/k6/lib/*.js load/k6/scenarios/*.js` passes
- `task --list` shows steady, mixed, spike, and guarded saturation commands
- `docker compose -f deploy/docker-compose/docker-compose.yml config` passes
- `task test` passes
- runtime scenario validation still pending successful Docker image pull
- saturation guard still needs runtime verification through `task load:saturation`

## Review Notes

- resolved review feedback for pacing, bounded labels, and limiter-safe create defaults; runtime validation pending image availability

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
