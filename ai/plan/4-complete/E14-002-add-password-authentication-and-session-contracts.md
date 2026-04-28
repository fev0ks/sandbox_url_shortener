# E14-002 - Add Password Authentication And Session Contracts

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Add email/password registration and login to identity, including password credential storage,
session issuance, session validation, and logout contracts for browser and gateway callers.

## Scope

- add account registration with normalized email, password hashing, and duplicate-account handling
- add password login with safe credential verification and generic failure responses
- add opaque session issuance, lookup, expiry, renewal rules, rotation on authentication, and
  logout invalidation
- add CSRF token issuance and verification for state-changing cookie endpoints (double-submit
  token pattern per epic decision)
- expose transport-safe HTTP endpoints for signup, login, current session, validation, logout, and
  CSRF token bootstrap
- define shared contract constants for session cookie/header names, CSRF cookie/header names, and
  identity claims returned to gateway or web callers
- emit structured audit log events for signup, login success, login failure, logout, and session
  revocation with stable fields
- exclude Google OAuth, Apple OAuth, and UI implementation

## Acceptance Criteria

- password hashes use `argon2id` with documented baseline parameters (or `bcrypt` cost ≥ 12 if
  argon2id is rejected during architect pass); algorithm and parameters are persisted with the
  hash so future upgrades are transparent; raw passwords are never logged or persisted
- password policy enforces a minimum length of 12 characters and a maximum length of 72 bytes to
  bound hash cost and align with bcrypt-compatible upgrade paths
- session cookies are set as `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`; the cookie value is an
  opaque random token of at least 128 bits, and only its hash is stored server-side
- a fresh session id is issued on every successful authentication (signup, login, OAuth completion
  in later tasks) and the prior session id is revoked, mitigating session fixation
- CSRF token is issued by identity, exposed via a non-`HttpOnly` companion cookie, and required as
  a header on every state-changing cookie-authenticated request; mismatch returns a stable
  authentication error
- signup and login return stable JSON responses and session material without exposing sensitive
  credential state
- session validation returns the authenticated account id, email, provider summary, and expiry in a
  gateway-safe contract
- logout invalidates the current session and repeated logout is safe
- expired, missing, malformed, and revoked sessions map to explicit authentication errors
- failed login attempts emit an audit log event with stable fields (account id when known,
  normalized email hash, request id, reason code) without logging the submitted password or full
  email
- service-level tests cover signup, duplicate email, successful login, bad password, expired
  session, revoked session, logout behavior, CSRF accept/reject, and session rotation on login

## Assumptions

- cookie-first opaque sessions are fixed by the epic; this task does not revisit the decision
- session storage belongs to identity and uses PostgreSQL in the first pass
- email verification is deferred; the schema may include verification fields but authentication
  must not silently depend on an unimplemented email flow
- per-IP and per-account login throttling is not built into this task; failure events must still be
  emitted in a shape that a future shared abuse-control layer can consume without contract changes
- absolute session lifetime defaults to 30 days and idle timeout to 7 days unless the architect
  pass narrows them; renewal extends `last_seen` and may slide `expires_at` within the absolute
  cap

## Implementation Notes

- keep password policy validation and credential orchestration in the service layer
- keep request parsing and response mapping in handlers only
- declare reused cookie names, header names, and external contract keys as constants
- use `context.Context` through public service and repository boundaries
- use structured logs for operational events without logging emails with passwords, raw tokens, or
  session secrets
- document the session validation contract clearly enough for E15 gateway implementation
- implemented signup, login, current-session, validation, logout, and CSRF bootstrap endpoints
- implemented Argon2id password hashing with persisted algorithm and JSON parameters
- sessions are opaque random tokens with only token hash stored server-side; successful auth
  rotates any prior session token supplied by cookie
- CSRF uses a double-submit companion cookie plus `X-CSRF-Token`

## Progress Checklist

- [x] confirm session mode, expiry, cookie attributes, and validation response shape
- [x] add password credential storage and hashing helpers behind testable boundaries
- [x] implement signup and password login service flows
- [x] implement session create, validate, and revoke flows
- [x] add HTTP handlers and DTOs for password auth and sessions
- [x] add unit and client tests for success, edge, and failure paths
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `gofmt -w services/identity`
- `go test ./...` passed
- `task lint` passed
- `go build ./services/identity/cmd/identity ./services/shortener/cmd/shortener ./services/analytics/cmd/analytics` passed
- `npm test` passed in `services/web`
- `npm run typecheck` passed in `services/web`
- `npm run build` passed in `services/web`

## Review Notes

- implementation complete; reviewed against handler, service, and acceptance criteria
- second-pass review after fixes: previous P1/P2/P3 findings are resolved; two low-priority
  follow-ups recorded below

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] `constantStringEqual` in
  [tokens.go:23-27](services/identity/internal/service/tokens.go:23) hashes both inputs and
  compares the resulting `[32]byte` arrays with `==`. Go does not specify constant-time array
  equality; the helper name promises something the runtime does not deliver. Use
  `subtle.ConstantTimeCompare(ah[:], bh[:]) == 1`, or compare the inputs directly with
  `subtle.ConstantTimeCompare` when both have known fixed length.
  Resolution: changed the helper to compare the fixed-size hashes with
  `subtle.ConstantTimeCompare`.
- [x] `Login` in
  [auth_service.go:84-103](services/identity/internal/service/auth_service.go:84) returns
  `ErrUnauthorized` immediately when the email is unknown, skipping any password hashing work.
  This leaks account existence by response-latency timing. Mitigation: always run a
  fixed-cost dummy `argon2.IDKey` against a constant precomputed hash before returning the
  unauthorized error so the unknown-account and bad-password paths take comparable time.
  Resolution: unknown-account login now runs a fixed-cost dummy Argon2id hash before returning
  the generic unauthorized error.
- [x] Audit logging gap. Scope and acceptance criteria require structured audit events for
  signup, login success, login failure, logout, and session revocation, but only login success
  and login failure pass through `s.audit` (see
  [auth_service.go:88-108](services/identity/internal/service/auth_service.go:88)). Signup
  ([auth_service.go:39-81](services/identity/internal/service/auth_service.go:39)) and logout
  ([auth_service.go:147-156](services/identity/internal/service/auth_service.go:147)) emit only
  generic info logs without `event` and `reason_code` fields.
  Resolution: signup, logout, failed logout, and session revocation now emit structured audit
  events through the shared audit helper.

### P2 Medium

- [x] CSRF bootstrap cookie issued by `GET /api/v1/auth/csrf` uses `MaxAge =
  absoluteSessionTTL` (~30 days) for unauthenticated callers (see
  [handler.go:91](services/identity/internal/transport/http/stdlib/auth/handler.go:91) and
  [handler.go:321](services/identity/internal/transport/http/stdlib/auth/handler.go:321)). A 30-day
  CSRF token for an anonymous browser is excessive; either tie the bootstrap CSRF cookie to a
  short window (e.g., 1 hour) or rotate it on every signup/login attempt regardless of outcome.
  Resolution: CSRF bootstrap now uses the explicit `CSRFBootstrapTTL` one-hour window.
- [x] `RevokeSessionByTokenHash` in
  [repository.go:138-147](services/identity/internal/repository/postgres/repository.go:138)
  silently succeeds when the token hash matches no row, so logout cannot distinguish "session
  was active" from "no session at all". Combined with the missing logout audit event this hides
  abuse signals; consider returning a sentinel and letting the service decide whether to audit.
  Resolution: repository revocation now returns whether a row changed; the service uses that for
  logout and session-revocation audit outcomes.

### P3 Low

- [x] `passwordParams.Salt` is base64-encoded inside JSON params next to the algorithm; works,
  but a dedicated `salt` column would make hash-format upgrades easier to reason about.
  Resolution: tracked as credential-format hardening in follow-up `E14-006`.
- [x] CSRF cookie is rotated on every successful authentication via
  [handler.go:274-280](services/identity/internal/transport/http/stdlib/auth/handler.go:274),
  but the issued value is never returned in the JSON body — clients must read it from the
  cookie. Document this on the session contract for E15 gateway and `services/web` consumers.
  Resolution: tracked as session contract documentation in follow-up `E14-006`.
- [x] `domain.LoginInput` carries `RequestID` and `NormalizedEmailLog` fields that are never
  populated by callers ([types.go:101-104](services/identity/internal/domain/types.go:101));
  same for `OAuthLoginInput.{RequestID, ClientIP, UserAgent}`
  ([types.go:118-126](services/identity/internal/domain/types.go:118)). Either wire them through
  the audit pipeline (request id is already available via `middleware.GetRequestID`) or remove
  them — dead struct fields invite future code that reads `nil` data and silently logs blanks.
  Resolution: removed dead `RequestID`/`NormalizedEmailLog` fields and wired OAuth callback
  `ClientIP`/`UserAgent` through to the OAuth login audit event.
- [x] OAuth callbacks issue identity sessions through `IssueOAuthSession` →
  `issueSession`, which only emits `session_revocation` (on rotation) and
  `auth_session_issue` (operational), but no `login_success` audit event for the OAuth path
  (see [auth_service.go:170-220](services/identity/internal/service/auth_service.go:170)).
  Scope says audit covers login_success generally; password login does this, OAuth does not.
  Add a single audit emission in `IssueOAuthSession` (or in the OAuth handler) so a Google/Apple
  sign-in produces a `login_success` event symmetrical to the password path.
  Resolution: `IssueOAuthSession` now emits `login_success` with provider reason code after
  successful account linking and session issuance.
