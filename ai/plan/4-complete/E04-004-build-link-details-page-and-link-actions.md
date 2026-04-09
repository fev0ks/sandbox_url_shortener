# E04-004 - Build Link Details Page And Link Actions

Status: complete

Epic: E04 - Web Frontend

## Summary

Build a link details page in the web frontend and connect it to the existing backend actions for
enable, disable, and delete so the UI covers the full current link lifecycle.

## Scope

- link details page in the web frontend
- fetch and render current link fields from backend
- browser actions for enable / disable / delete
- navigation from dashboard to link details
- excludes analytics dashboards beyond current backend fields
- excludes auth and role-based permissions

## Acceptance Criteria

- a link details page can be opened from the dashboard
- the page renders current backend fields clearly, including short URL and lifecycle state
- enable / disable / delete actions work from the browser and refresh the UI state correctly
- destructive actions have clear UX feedback and do not silently fail
- the page does not invent stats that the backend does not actually provide

## Assumptions

- existing `GET /api/v1/links/{id}` and lifecycle endpoints are sufficient for the first version
- action UX can be simple confirm / success / error feedback without modal-heavy complexity
- link details can show current `redirect_count` as stored by the backend without pretending it is
  a full analytics view

## Implementation Notes

- keep lifecycle actions aligned with existing backend contracts instead of creating parallel
  frontend-only mutations
- reuse the frontend API client layer instead of issuing raw fetches from multiple components
- state refresh after actions should be explicit and predictable

## Progress Checklist

- [x] confirm scope and assumptions
- [x] build frontend details page
- [x] integrate enable / disable / delete actions
- [x] add navigation from links dashboard
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `npm run test`
- `npm run typecheck`
- `npm run build`
- Playwright MCP smoke against live stack:
  - open `/links`
  - open details page for a live link
  - verify details view renders backend fields
  - disable link and confirm success notice + disabled state
  - enable link and confirm success notice + active state
  - delete link and confirm redirect to dashboard + deleted notice + item removed from list
- local integration against running shortener backend on `http://localhost:18080`

## Review Notes

- details page is server-rendered at `/links/[id]`
- lifecycle actions use Next server actions and redirect back with explicit notice params
- action flow fixed to avoid catching successful `redirect(...)` as an error inside `try/catch`

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
