# E11-003 - Add Postgres And Redis Exporters

Status: review

Epic: E11 - Load And Stress Testing

## Summary

Add local PostgreSQL and Redis exporter services so load tests can be interpreted against database
and cache behavior instead of only application-level HTTP metrics.

## Scope

- add PostgreSQL exporter and Redis exporter services to the Docker Compose stack
- configure exporter connectivity to the existing local `postgres` and `redis` services
- add Prometheus scrape jobs for both exporters
- document the new Prometheus targets and the minimum useful metrics to inspect during load runs
- exclude tuning PostgreSQL or Redis settings unless required for exporter connectivity
- exclude dashboard construction, which belongs to `E11-004`

## Acceptance Criteria

- `task deploy` starts the exporters with the local stack
- Prometheus target discovery shows healthy PostgreSQL and Redis exporter targets
- exporter configuration does not log secrets or expose sensitive payloads
- docs list the key infrastructure signals to watch during load tests, such as DB connections,
  query pressure, Redis ops, memory, and evictions
- the implementation remains local-only and does not introduce cloud or external monitoring

## Assumptions

- Docker Compose remains the local runtime for this epic
- exporter image tags should be pinned during implementation rather than left floating
- the existing local database credentials are acceptable for local exporter connectivity

## Implementation Notes

- keep exporter services under `deploy/docker-compose/docker-compose.yml`
- keep scrape configuration in `deploy/docker-compose/prometheus/prometheus.yml`
- prefer read-only or least-privilege exporter settings where the selected exporter supports them
- verify exporter target names are stable because dashboards in `E11-004` will depend on them

## Progress Checklist

- [x] confirm scope and assumptions
- [x] choose pinned exporter images
- [x] add exporter services to Docker Compose
- [x] add Prometheus scrape jobs
- [x] document target checks and key metrics
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `docker compose -f deploy/docker-compose/docker-compose.yml config` passes
- `task test` passes
- `task deploy` attempted, but Docker pull hangs on exporter images
- still needs Prometheus target validation for `postgres-exporter` and `redis-exporter`

## Review Notes

- compose validation still passes after review fixes; runtime validation pending image availability

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
