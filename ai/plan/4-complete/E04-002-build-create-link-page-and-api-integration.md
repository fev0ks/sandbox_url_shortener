# E04-002 - Build Create Link Page And API Integration

Status: complete

Epic: E04 - Web Frontend

## Summary

Build the first real user flow in the web app: a create-link page that submits to the shortener
API, validates user input, and presents the created short URL in a browser-friendly way.

## Scope

- create-link page in `services/web`
- shared API client for `POST /api/v1/links`
- form validation and submission states
- success state with created short URL and copy action
- excludes links dashboard
- excludes auth and user accounts

## Acceptance Criteria

- the web app exposes a create-link page reachable from the root navigation
- submitting a valid URL creates a link through the existing shortener API
- validation and server errors are shown as usable UI feedback, not raw JSON blobs
- success state clearly shows the short URL and key fields returned by the backend
- frontend does not duplicate backend business rules beyond basic UX validation

## Assumptions

- existing `POST /api/v1/links` response contract is stable enough for the first page
- browser clients can consume the current API shape without requiring a dedicated BFF first
- copy-to-clipboard can be a lightweight browser enhancement without extra infrastructure

## Implementation Notes

- keep API calls centralized in a small frontend client layer
- define typed DTO mapping in the frontend rather than sprinkling anonymous fetch code across pages
- decide explicitly whether submission happens via server actions, route handlers, or direct fetch;
  document the choice in the task as part of implementation
- keep the form UX pragmatic: loading, field errors, generic backend error, success

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement frontend API client for create flow
- [x] build create page and form states
- [x] add success state and copy action
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `npm run test` in `services/web` — passes (`vitest`)
- `npm run typecheck` in `services/web` — passes
- `npm run build` in `services/web` — passes
- `curl http://localhost:3000` against the dev server returns the rendered create page markup
- `curl -X POST http://localhost:8080/api/v1/links ...` returns `201`-style JSON from the live
  shortener backend
- full browser-submit smoke was not automated because Playwright in this environment fails to
  initialize its work dir (`/.playwright-mcp` is read-only)

## Review Notes

- chose a Next.js server action instead of direct browser fetch so the page can integrate with
  the existing shortener API without introducing a CORS task into `E04-002`
- added a small typed API client and a narrow form-normalization layer, then covered that layer
  with `vitest` tests
- root route is now the real create-link page with form validation, submission state, success
  card, and copy-to-clipboard action

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
