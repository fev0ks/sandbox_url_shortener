# E11-004 - Build Load Testing Dashboards

Status: review

Epic: E11 - Load And Stress Testing

## Summary

Create a Grafana dashboard focused on load-test interpretation by combining k6 test-driver metrics,
shortener application metrics, event-pipeline metrics, and PostgreSQL/Redis exporter metrics.

## Scope

- add a provisioned Grafana dashboard JSON for load testing
- include k6 throughput, latency, and failure panels from `E11-002`
- include shortener HTTP, create, redirect, cache, and publish panels from existing app metrics
- include PostgreSQL and Redis panels from `E11-003`
- document how to use the dashboard during steady, mixed, and spike runs
- exclude alerting rules and paging policies
- exclude broad redesign of existing `Shortener Overview` or `Analytics Overview` dashboards

## Acceptance Criteria

- Grafana provisions a dedicated load-testing dashboard from repo-owned JSON
- the dashboard shows k6, application, and infrastructure metrics on one screen
- panels answer concrete operational questions such as "is latency app-side or infra-side" and
  "did Redis stop absorbing redirect load"
- docs describe the intended reading order during a load run
- dashboard variables and labels stay bounded and do not rely on per-link labels

## Assumptions

- `E11-002` has established stable k6 metric names or validation queries
- `E11-003` has established stable exporter target names
- dashboard provisioning continues through `deploy/docker-compose/grafana/dashboards`

## Implementation Notes

- add a new dashboard instead of overloading the existing overview dashboards
- use conservative PromQL that remains meaningful on a local laptop with short test windows
- prefer p95/p99 latency, rate, error ratio, cache outcomes, and exporter pressure signals over
  decorative panel count
- include links or notes in docs for jumping between the load dashboard and existing service
  dashboards

## Progress Checklist

- [x] confirm scope and assumptions
- [x] identify final k6 and exporter metric names
- [x] add the provisioned load-testing dashboard
- [x] document the dashboard workflow
- [x] validate dashboard JSON and static queries
- [x] pass review or resolve review findings

## Validation

- `jq empty deploy/docker-compose/grafana/dashboards/load-testing-overview.json` passes
- `docker compose -f deploy/docker-compose/docker-compose.yml config` passes
- dashboard k6 latency queries use the documented remote-write millisecond suffix
- `task test` passes
- runtime provisioning and live panel validation still pending successful Docker image pull and stack restart

## Review Notes

- resolved review feedback for k6 remote-write latency query names; runtime provisioning and live panel validation pending image availability

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
