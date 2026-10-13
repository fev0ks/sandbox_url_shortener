# Streamable HTTP Runtime And Local Lifecycle

Status: complete

Epic: E21 - MCP Agent Investigation Tools

## Summary

Run MCP independently of its clients over authenticated Streamable HTTP while retaining stdio.

## Scope

- Official Go MCP SDK stateful HTTP transport, sessions, cancellation and protocol negotiation.
- Injected immutable development key registry mapping issued bearer keys to client identities.
- Host/Origin validation, bounded request bodies, shallow health and graceful shutdown.
- Service Dockerfile, loopback Compose endpoint and independent Task lifecycle commands.

## Acceptance Criteria

- Real SDK clients initialize, discover tools and call existing investigation tools.
- Missing/invalid/revoked keys cannot initialize, list, call, GET or DELETE sessions.
- Sessions cannot cross client identities; stale sessions fail and clients can reinitialize.
- Restarting only MCP preserves its URL; unhealthy PostgreSQL does not prevent readiness.

## Assumptions

- Local-only development registry; key changes require rebuild/restart.
- Clients may need to refresh a changed tool catalog after reconnecting.

## Implementation Notes

- SDK v1.6.1 supports Go 1.25; its bearer middleware carries verified UserID into sessions.
- JSON HTTP responses, 15-minute idle sessions; existing stdio dispatcher is reused.
- Endpoint http://localhost:8090/mcp; health endpoints check transport readiness only.

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task test`, `task build`, and `task lint`: passed with `GOTOOLCHAIN=go1.25.0`; lint reports zero issues.
- Docker CGO-disabled image build, `task mcp:start`, `task mcp:restart`, status and full-stack `task smoke`: passed.
- Real HTTP initialization negotiated 2025-11-25 and exposed all nine tools; logs, request-chain and Tempo trace calls passed.
- Restart invalidated the prior session (404); fresh initialization and tool calls passed at the same URL.
- API-key/auth/session ownership, cancellation, deadline, partial-body shutdown and race regressions passed.
- With PostgreSQL paused, database calls returned bounded errors within 5 seconds, health stayed 200 and log tools worked.
- Existing local Codex registration switched to Streamable HTTP with its issued key; client configuration refresh is needed for an existing connection.

## Review Notes

- Final changed-files review: APPROVED; all blocking findings resolved with regressions.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] partial initialization cannot hold shutdown indefinitely; active calls are cancelled before closing sessions
- [x] transaction-scoped deadlines reach query and result reads
- [x] column/field/byte limits apply while scanning, before oversized values can accumulate

### P3 Low

- [x] no open P3 findings
