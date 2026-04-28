# E14-005 - Add Web Auth Flows And Session Aware Navigation

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Add browser auth screens and session-aware navigation in `services/web` for password login,
signup, Google login, Apple login, logout, and current-user display.

## Scope

- add signup, login, OAuth entry, OAuth callback result, logout, and current-session UI flows in
  `services/web`
- call `services/identity` directly until E15 ships; switching the base URL to the gateway is the
  responsibility of `E15-003` and must require only a single configuration change in `web`
- bootstrap and attach the CSRF token defined by `E14-002` on every state-changing request
- show session-aware navigation states for anonymous and authenticated users
- show an explicit account-linking message when the API returns the epic-default linking rule
  outcome (matching unverified or missing email), instead of silently creating or merging an
  account
- preserve the existing link creation and dashboard workflows while auth is introduced
- handle loading, validation, authentication failure, provider failure, CSRF rejection, and
  logout states
- exclude owned-link enforcement and personal analytics views; those belong to E16

## Acceptance Criteria

- anonymous users can reach signup and login screens and choose password, Google, or Apple sign-in
- successful auth updates the browser session state and navigation without requiring a manual page
  refresh
- logout clears local session state and returns the UI to anonymous navigation
- auth failures show transport-safe messages without exposing backend internals or sensitive values
- existing public pages and link workflows still render after auth navigation is added
- frontend tests or browser-level checks cover signup/login rendering, provider button flows,
  session-aware navigation, and logout behavior

## Assumptions

- `services/web` remains the browser frontend runtime for this task
- the API base URL is identity directly in v1 and gateway in a later milestone; the migration to
  gateway is owned by `E15-003`, so this task only requires a single configurable base URL
- session cookies are `HttpOnly` per `E14-002`, so the UI relies on a current-session endpoint
  rather than reading session material directly
- the CSRF companion cookie is non-`HttpOnly` by design so the UI can read it and forward it as a
  request header
- account ownership changes for links are deferred to E16

## Implementation Notes

- keep auth API client code inside `services/web` and avoid leaking backend service URLs through
  scattered component code
- use existing web design and routing conventions rather than introducing a new UI framework
- model current-user state as a small explicit client boundary that can be reused by E16 and E17
- avoid visible implementation instructions in the UI; make controls self-explanatory
- verify responsive layouts for auth forms and navigation states before moving the task to review
- implemented `/auth` password signup/login UI, Google and Apple provider entry links,
  `/auth/callback` provider result page, session-aware navigation, CSRF bootstrap, and logout
- added a single configurable identity API base URL via `NEXT_PUBLIC_IDENTITY_API_BASE_URL`

## Progress Checklist

- [x] confirm API target, route names, and session-state contract
- [x] add signup, login, OAuth entry, callback result, and logout flows
- [x] add current-session loading and session-aware navigation
- [x] preserve existing link and dashboard behavior
- [x] add frontend tests or browser checks for auth workflows
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `npm test` passed in `services/web`
- `npm run typecheck` passed in `services/web`
- `npm run build` passed in `services/web`
- Browser smoke loaded `http://localhost:3002/auth` against a local identity API mock and covered
  signup rendering, signup success, provider start links, session-aware navigation, login, logout,
  and auth-panel session reset.

## Review Notes

- implementation complete; reviewed against `services/web` auth client, panel, and callback page
- second-pass review after fixes: previous P1/P2/P3 findings are resolved; no new findings

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] Validation evidence does not satisfy the acceptance criterion that "frontend tests or
  browser-level checks cover signup/login rendering, provider button flows, session-aware
  navigation, and logout behavior". The recorded Playwright run only loaded `/auth` against a
  not-running identity backend; signup, login, OAuth navigation, and logout were not exercised
  end to end. Either run identity during the smoke check (covering at least signup → session →
  logout) or explicitly add a follow-up task and mark this gap on the card.
  Resolution: reran browser smoke with a local identity API mock covering signup, login, provider
  links, session-aware nav, and logout behavior.

### P2 Medium

- [x] `bootstrapCSRF` is invoked on every `passwordRequest` and `logout` call (see
  [identity-api.ts:97-128](services/web/lib/identity-api.ts:97)). Each call rotates the CSRF
  cookie on the server, which can desynchronize state when multiple tabs share the same
  identity session. Cache the bootstrap result for the lifetime of the page or only refetch on
  `invalid_csrf` responses.
  Resolution: CSRF bootstrap is cached per page lifetime and refetched only after
  `invalid_csrf`.
- [x] `defaultIdentityApiBaseUrl = "http://localhost:8081"` is hardcoded in
  [identity-api.ts:1](services/web/lib/identity-api.ts:1). Accept it as a dev fallback, but
  fail loud (or refuse to start) when `NEXT_PUBLIC_IDENTITY_API_BASE_URL` is unset in non-local
  builds; otherwise a deploy without the env var silently points the browser at localhost.
  Resolution: localhost remains a local-dev fallback only; non-local browser hosts now fail loud
  when `NEXT_PUBLIC_IDENTITY_API_BASE_URL` is missing.
- [x] On successful signup/login, the panel sets in-memory state but does not refresh the
  session-aware navigation rendered server-side; the user sees "Signed in" inside the panel
  while the page nav still shows the anonymous state until a manual reload. Trigger a router
  refresh or hoist the session state into the layout.
  Resolution: successful signup/login dispatches a shared session-change event and triggers a
  router refresh; navigation and auth panel both subscribe to current-session changes.

### P3 Low

- [x] `messageForError` in
  [auth/callback/page.tsx:47-58](services/web/app/auth/callback/page.tsx:47) duplicates
  `messageForCode` from
  [auth-panel.tsx:132-143](services/web/components/auth-panel.tsx:132). They will drift; move
  the mapping into `lib/identity-api.ts` (or a sibling module) and import from both places.
  Resolution: moved identity error message mapping into `services/web/lib/identity-api.ts` and
  imported it from both UI call sites.
