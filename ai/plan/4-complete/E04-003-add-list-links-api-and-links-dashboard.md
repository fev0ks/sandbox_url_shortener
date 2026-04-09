# E04-003 - Add List Links API And Links Dashboard

Status: complete

Epic: E04 - Web Frontend

## Summary

Add the missing backend contract for listing links, then build a basic dashboard page in the web
frontend so the UI is not limited to "create one link and lose it".

## Scope

- new backend endpoint for listing recent links
- repository / service / transport support in shortener
- frontend dashboard page consuming the new endpoint
- basic empty state and recent-links presentation
- excludes filtering, search, and pagination unless required for correctness
- excludes analytics charts

## Acceptance Criteria

- the backend exposes a list endpoint with a stable response contract for the frontend
- the web app renders a recent-links page using that endpoint
- empty state is explicit when no links exist
- list items include enough information to navigate to the link details page
- the implementation does not introduce frontend-only data paths that bypass the service layer

## Assumptions

- a simple recent-first query is enough for the first dashboard
- pagination can be deferred until the list is large enough to demand it
- list response should be browser-friendly and not force the frontend to reconstruct state from
  multiple backend calls

## Implementation Notes

- this task intentionally includes both backend and frontend because the frontend requirement
  exposes a missing contract in the API surface
- keep the list API intentionally narrow: recent links, stable fields, no speculative filters
- document the response shape if it becomes a durable frontend contract

## Progress Checklist

- [x] confirm scope and assumptions
- [x] add backend list endpoint
- [x] add or update backend tests
- [x] build frontend recent-links page
- [x] add frontend empty state and navigation
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./services/shortener/internal/service ./services/shortener/internal/transport/http/stdlib/link`
  — passes
- `task test` — passes
- `npm run test` in `services/web` — passes
- `npm run typecheck` in `services/web` — passes
- `npm run build` in `services/web` — passes (`/links` is dynamic and builds cleanly)
- `curl http://localhost:18080/api/v1/links` returns a list payload with recent items
- Playwright smoke against `http://localhost:3000/links` shows a non-empty dashboard populated from
  the live shortener API

## Review Notes

- backend now exposes `GET /api/v1/links` as a narrow recent-first list contract:
  `{"items":[...]}` using the existing link response shape
- frontend dashboard lives at `/links` and uses a server-rendered fetch path instead of browser
  CORS calls
- added empty state plus graceful API-error state so the page does not collapse into a raw runtime
  failure when the backend is unavailable

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
