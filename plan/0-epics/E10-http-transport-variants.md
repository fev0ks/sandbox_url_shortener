# E10 - HTTP Transport Variants

## Summary

Optional exploration epic for comparing alternative Go HTTP transports after the main stdlib-based
runtime is stable.

## Goal

Add one or more alternative HTTP entrypoints such as `api-http-chi` and `api-http-gin` without
forking business logic, so the project can compare developer ergonomics and tradeoffs on the same
feature set.

## Scope

- alternative `cmd/api-http-*` entrypoints
- framework-specific transport adapters under `internal/transport/http/*`
- reuse of shared service, repository, config, logger, and middleware concepts
- practical comparison notes for routing, middleware, request decoding, and testing
- excludes changing the canonical MVP runtime away from `api-http-stdlib` unless a later explicit
  decision is made

## Key Decisions

- `cmd/api-http-stdlib` remains the canonical baseline
- alternative variants are added only after the main functional path is stable
- differences must stay in transport and bootstrap layers, not leak into service or repository code
- comparisons should be done on the same endpoints and middleware concerns

## Task Links

- `E10-001-extract-shared-http-bootstrap-boundaries.md`
- `E10-002-add-api-http-chi-entrypoint.md`
- `E10-003-add-api-http-gin-entrypoint.md`
- `E10-004-document-http-transport-comparison.md`

## Open Questions

- whether `chi` should be implemented before `gin`, or only one alternative is enough
- whether experimental binaries live directly under `cmd/` or under a dedicated `cmd/experiments/`
