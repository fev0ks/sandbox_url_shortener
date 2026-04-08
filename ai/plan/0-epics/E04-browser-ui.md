# E04 - Browser UI

## Summary

User-facing epic for making the service usable from a browser without relying on Postman.

## Goal

Ship a simple, server-friendly UI for creating links, browsing recent links, and viewing link
details with basic feedback.

## Scope

- create link page
- recent links page
- link details and basic stats page
- copy short URL action and lightweight UX feedback
- excludes heavy SPA architecture and advanced frontend state management

## Key Decisions

- prefer server-rendered pages or a minimal frontend over a SPA
- keep UI aligned with existing backend contracts instead of inventing separate frontend-only APIs
- usability matters more than visual complexity for the first version

## Task Links

- `E04-001-build-create-link-page.md`
- `E04-002-build-recent-links-page.md`
- `E04-003-build-link-details-and-stats-page.md`
- `E04-004-add-basic-ux-feedback-and-copy-actions.md`

## Open Questions

- whether HTML templates live under `internal/ui` or `web/templates` in the initial implementation
- how much of the stats page should be stubbed before ClickHouse is integrated
