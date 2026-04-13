# E07-001 - Add Elasticsearch Kibana And Log Shipping

Status: complete

Epic: E07 - Log Observability

## Summary

Add the local log observability stack so application JSON logs flow through Fluent Bit into Elasticsearch and can be inspected in Kibana.

## Scope

- Docker Compose services for Elasticsearch, Kibana, and Fluent Bit
- Fluent Bit pipeline configuration for `shortener` and `analytics`
- local log routing from container stdout to searchable Elasticsearch indices
- excludes log field normalization inside application code

## Acceptance Criteria

- local Compose can start Elasticsearch, Kibana, and Fluent Bit alongside the app stack
- application logs from `shortener` and `analytics` reach Elasticsearch through Fluent Bit
- Kibana is reachable locally and can browse the shipped log indices
- docs reflect the local log stack and ownership boundaries

## Assumptions

- local Docker log shipping is enough for the MVP observability baseline
- one shared local index pattern is acceptable before retention and ILM tuning

## Implementation Notes

- keep application logs as JSON on stdout; do not add file-based logging
- Fluent Bit should parse JSON and preserve structured fields rather than flattening everything into raw text
- keep service build/runtime ownership in `services/*` and orchestration in `deploy/docker-compose`

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- local smoke: app logs appear in Elasticsearch and are visible in Kibana

## Review Notes

- Compose now defines `elasticsearch`, `kibana`, and `fluent-bit`, and `shortener`/`analytics`
  ship stdout JSON through the Docker `fluentd` logging driver to Fluent Bit.
- Local docs now describe the Compose shipping path `stdout -> fluentd driver -> Fluent Bit ->
  Elasticsearch -> Kibana`.
- `docker compose ... config` passes.
- live smoke completed: app logs are visible in `shortener-logs-*`, Elasticsearch is reachable on
  `:9200`, and Kibana is reachable on `:5601`.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
