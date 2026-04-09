# E04 - Web Frontend

Status: complete

## Summary

Build a dedicated web frontend as a separate service instead of embedding HTML templates into the
Go shortener runtime.

## Goal

Ship a usable browser-facing app on `Next.js + TypeScript` that talks to the shortener HTTP API,
supports the core link lifecycle, and gives the project a clean frontend/backend split.

## Scope

- new frontend service under `services/web`
- `Next.js` app shell, layout, local dev wiring, and environment contract
- create-link page with API integration and browser-friendly feedback
- links dashboard / recent links page
- link details page with enable / disable / delete actions
- shared frontend API client and typed request / response mapping
- minimal design system / styling baseline suitable for later growth
- excludes auth
- excludes analytics dashboards and advanced charts
- excludes moving HTML rendering into the Go backend

## Key Decisions

- frontend is a separate runtime: `services/web`
- backend stays API-first; no Go HTML template layer should be introduced for this epic
- prefer `Next.js + TypeScript` over ad-hoc static HTML so the UI can grow into a real product
- backend-facing gaps required by the UI should be handled explicitly, not hidden inside frontend
  tasks
- recent links UI requires a list endpoint; that contract should be added deliberately rather than
  stitched together client-side from ad-hoc IDs

## Task Links

- `E04-001-scaffold-nextjs-web-service.md`
- `E04-002-build-create-link-page-and-api-integration.md`
- `E04-003-add-list-links-api-and-links-dashboard.md`
- `E04-004-build-link-details-page-and-link-actions.md`

## Open Questions

- whether the frontend should call the shortener API directly from the browser in local dev or
  use `Next.js` route handlers / server actions as a thin BFF layer
- whether the initial UI should use a component library or stay on plain CSS / Tailwind-level
  primitives
- whether list pagination is needed immediately or if a simple recent-links page is enough for
  the first version

## Completion Note

`services/web` is now a working standalone frontend service with create flow, recent-links
dashboard, and link details / lifecycle actions wired to the current shortener API.
