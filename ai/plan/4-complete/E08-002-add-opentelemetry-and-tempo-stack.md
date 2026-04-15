# E08-002 - Add OpenTelemetry And Tempo Stack

Status: complete

Epic: E08 - Metrics And Dashboards

## Summary

Add the tracing backbone: OpenTelemetry SDK wiring in services and a local `Tempo` backend for OTLP
trace export.

## Scope

- local `Tempo` service in Compose
- shared OTLP/trace config shape for `shortener` and `analytics`
- base OpenTelemetry tracer provider setup and exporter wiring
- excludes actual application spans beyond minimal bootstrap proof
- excludes Grafana dashboards

## Acceptance Criteria

- local Compose starts `Tempo`
- `shortener` and `analytics` can export traces via OTLP to Tempo
- app startup/shutdown remains healthy when tracing is enabled
- docs reflect trace backend ownership and local access points

## Assumptions

- OTLP over HTTP or gRPC is acceptable for the local stack; exact protocol can be chosen during implementation
- tracing must degrade gracefully if Tempo is temporarily unavailable

## Implementation Notes

- prefer one small shared platform package for tracer setup instead of service-local duplication
- keep tracing config service-owned even if helper code is shared
- choose resource attributes deliberately: `service.name`, `service.version`, `deployment.environment`

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add Tempo to local stack
- [x] add shared OpenTelemetry bootstrap helper
- [x] wire `shortener` and `analytics` exporter config
- [x] update docs and local access notes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./...` for touched packages
- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- local smoke: traces appear in Tempo after a live request/consumer event

## Review Notes

- shared `internal/platform/telemetry` now owns OTLP bootstrap and HTTP tracing middleware
- local `tempo` is healthy and accepts OTLP HTTP export from both services
- Tempo search API returns traces for both `service.name=shortener` and `service.name=analytics`
- health and `/metrics` routes are now excluded from HTTP tracing, so Tempo defaults stay focused
  on real app and consumer flows


## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] HTTP tracing now skips `/metrics` and `/health/*`, so the local Tempo store no longer fills
  with Prometheus scrapes and Docker health checks by default.

### P3 Low

- [x] no open P3 findings
