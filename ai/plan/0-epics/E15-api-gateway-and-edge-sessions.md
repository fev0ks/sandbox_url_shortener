# E15 - API Gateway And Edge Sessions

Status: active

## Summary

Edge architecture epic for unifying public HTTP access behind one gateway and centralizing
session-aware request handling.

## Goal

Add a `gateway` service that becomes the single public entrypoint for browser clients and routes
authenticated traffic to `identity`, `shortener`, and `analytics`.

## Scope

- `services/gateway` runtime
- routing to backend services
- auth and session validation at the edge
- identity propagation headers or claims for downstream services
- unified browser-facing API surface for `services/web`
- excludes domain workflows that belong to backend services

## Key Decisions

- gateway is mandatory and introduced before broad auth and UI expansion
- gateway stays thin and must not absorb business logic
- `services/web` should call gateway, not backend services directly
- SSE proxying support belongs here because realtime stats cross the edge boundary

## Task Links

- `E15-001-bootstrap-gateway-service-and-route-map.md`
- `E15-002-add-session-validation-and-identity-propagation.md`
- `E15-003-migrate-web-api-access-to-gateway.md`
- `E15-004-add-sse-proxy-support-for-realtime-stats.md`

## Open Questions

- whether gateway owns cookie issuance or only validation and forwarding
- how much response normalization should happen at the edge
