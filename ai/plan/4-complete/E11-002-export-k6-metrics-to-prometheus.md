# E11-002 - Export k6 Metrics To Prometheus

Status: review

Epic: E11 - Load And Stress Testing

## Summary

Wire k6 run metrics into the local Prometheus/Grafana observability loop so test-driver latency,
throughput, and failure signals can be compared with application metrics during a load run.

## Scope

- confirm the current k6 Prometheus-compatible output mode before implementation
- add the required k6 runner configuration for exporting metrics to the local metrics stack
- update Prometheus or the compatible receiver configuration as needed
- add task commands or environment defaults for running load scenarios with metrics export enabled
- document how to verify that k6 metrics arrive in Prometheus
- exclude Grafana dashboard construction, which belongs to `E11-004`
- exclude external hosted metrics backends

## Acceptance Criteria

- at least one k6 scenario from `E11-001` exports metrics into the local observability stack
- Prometheus can query k6 request rate, latency, and failure metrics during or immediately after a
  run
- the k6 metrics labels remain bounded and do not include `short_code`, `link_id`, request ids, or
  URLs with unbounded values
- docs explain the selected k6 metrics export mode and how to validate it locally
- the implementation keeps the default target local or compose-network scoped

## Assumptions

- the local Prometheus service remains the first metrics sink
- version-specific k6 output details must be verified during implementation rather than assumed
- a compose-network k6 runner may be simpler than a host-level k6 process once metrics export is
  added

## Implementation Notes

- evaluate the current supported k6 Prometheus path first, then choose the least complex option
  that works with the existing Docker Compose stack
- prefer explicit environment variables in Taskfile commands over hidden defaults embedded in
  scripts
- if Prometheus receiver flags are required, keep them local-stack-only and document why they
  exist
- add a small Prometheus validation query list to the docs so dashboard work can build on known
  metric names

## Progress Checklist

- [x] confirm scope and assumptions
- [x] verify the current k6 Prometheus export option
- [x] wire k6 metrics into the local stack
- [x] add or update task commands for metrics-enabled runs
- [x] document validation queries and operational usage
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- official Grafana k6 docs checked: local flow uses `experimental-prometheus-rw` and Prometheus remote write receiver
- `docker compose -f deploy/docker-compose/docker-compose.yml config` passes
- `task --list` shows metrics-enabled load tasks
- `node --check load/k6/lib/*.js load/k6/scenarios/*.js` passes
- `task test` passes
- runtime validation still needs a successful `task load:steady:metrics` and Prometheus query after Docker images are available

## Review Notes

- resolved review feedback for bounded k6 labels and Prometheus remote-write trend metric names; runtime validation pending image availability

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] Redirect metrics still carry dynamic short-code URLs
  - Redirect requests use `/${shortCode}` and only add an `endpoint` tag.
  - k6 emits URL/name tags by default, so the Prometheus remote-write series will still include
    per-short-code paths, violating the acceptance criterion that k6 metrics labels remain bounded
    and do not include unbounded URLs.
  - Add stable request naming or URL grouping, and consider trimming `systemTags` so the raw `url`
    tag does not persist dynamic paths.
  - Resolution: k6 scenarios now trim `systemTags` to exclude `url` and override request names
    with bounded values such as `GET /:shortCode`.
  - Source: `load/k6/lib/http.js:38`

### P3 Low

- [x] no open P3 findings
