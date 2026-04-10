# E18 - Realtime Stats Transport Variants

Status: active

## Summary

Realtime delivery epic for live statistics updates on open pages without manual refresh.

## Goal

Add a production path for live stats updates using `SSE` and then compare it with an experimental
`WebSocket` variant without changing domain logic.

## Scope

- live stats streaming contract
- owner and admin authorized realtime updates for stats pages
- gateway support for stream transport
- web client subscription handling and reconnect behavior
- experimental WebSocket path after SSE is stable
- excludes generic chat-style or bidirectional realtime features

## Key Decisions

- `SSE` is the canonical first implementation
- `WebSocket` is an experiment and follow-up, not the initial default
- realtime transport must not move analytics business logic into gateway
- UI changes for live stats belong inside this epic

## Task Links

- `E18-001-add-sse-stats-stream-endpoint.md`
- `E18-002-build-live-stats-ui-subscription-flow.md`
- `E18-003-add-reconnect-and-auth-failure-handling.md`
- `E18-004-evaluate-websocket-transport-variant.md`

## Open Questions

- whether live updates stream aggregates, deltas, or full snapshots
- whether the same stream contract serves both user and admin stats screens
