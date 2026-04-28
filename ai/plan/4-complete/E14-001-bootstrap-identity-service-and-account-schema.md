# E14-001 - Bootstrap Identity Service And Account Schema

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Create the first-pass `services/identity` runtime and storage model that owns accounts, profile
basics, and authentication method records for password and OAuth login.

## Scope

- add `services/identity` with a stdlib HTTP runtime, config, logging, health, metrics, and DB
  wiring consistent with existing backend services
- add identity-owned PostgreSQL migrations for the full v1 schema set: `accounts`,
  `auth_identities` (provider bindings), `password_credentials`, and `sessions`
- define domain, service, repository, and handler package boundaries for identity
- expose minimal readiness and local runtime wiring through existing Docker Compose and Taskfile
  conventions when needed
- exclude password login, OAuth callback handling, and web screens; those belong to later E14
  tasks

## Acceptance Criteria

- `services/identity/cmd/identity` starts with project logging, config loading, health/readiness,
  metrics, and graceful shutdown
- identity database migrations create durable tables for `accounts`, `auth_identities`,
  `password_credentials`, and `sessions`
- `accounts` carries stable internal id, normalized email (citext or lowercased canonical column
  with unique index), display name, lifecycle status (`active`, `disabled`, `deleted`),
  `email_verified_at` placeholder column (nullable, unused in v1), and standard timestamps
- `auth_identities` carries `(provider, provider_subject)` with a unique constraint, foreign key to
  `accounts`, and provider-specific metadata column reserved for OAuth-only fields
- `password_credentials` is a separate row per account that owns password material (hash, algo
  identifier, params, updated_at); password fields never live on `accounts`
- `sessions` carries opaque session id (or hash of the cookie value), foreign key to `accounts`,
  issued/expires/last_seen timestamps, revoked flag, and a stable index on the lookup column
- account records can represent email/password users, Google users, and Apple users without
  duplicating user profile ownership
- repository interfaces and implementations keep SQL out of handlers and business decisions out of
  repositories
- local developer commands and orchestration know how to build and run the identity service
- focused tests cover config loading and repository schema assumptions that can be tested locally

## Assumptions

- identity is a separate service under `services/identity`, not an internal package inside
  `shortener` or `gateway`
- PostgreSQL remains the source of truth for account and auth identity state
- account email verification is not required in v1; only an `email_verified_at` placeholder column
  is included so later flows can adopt it without a follow-up migration
- service-to-service auth is out of scope; E15 will define edge validation and downstream identity
  propagation
- session storage lives in identity-owned PostgreSQL per the epic; no Redis or in-memory store in
  the bootstrap slice

## Implementation Notes

- mirror the existing composition-root style from `services/shortener/internal/app`
- keep service-owned config in `services/identity/internal/config` and migrations in
  `services/identity/migrations`
- model an account as the stable internal user id, with separate auth identity rows for password,
  Google, and Apple provider bindings
- prefer explicit unique constraints for normalized email and provider subject pairs
- avoid introducing queues, caches, or background jobs in this bootstrap slice
- implemented `services/identity/cmd/identity` with config, structured logging, tracing, health,
  readiness, metrics, CORS for the web origin, and graceful shutdown
- added identity-owned Postgres migrations for `accounts`, `auth_identities`,
  `password_credentials`, `sessions`, and `oauth_states`
- wired local Docker Compose, Taskfile, Makefile, Prometheus, and image build paths for identity

## Progress Checklist

- [x] confirm service boundaries, schema fields, and local runtime assumptions
- [x] add identity composition root, config, logger, health, readiness, and metrics wiring
- [x] add account and auth identity migrations
- [x] add repository interfaces and PostgreSQL implementation skeleton
- [x] wire local build/run/migration entrypoints
- [x] add focused tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `gofmt -w services/identity`
- `go test ./...` passed
- `task lint` passed
- `go build ./services/identity/cmd/identity ./services/shortener/cmd/shortener ./services/analytics/cmd/analytics` passed
- `docker compose -f deploy/docker-compose/docker-compose.yml config` passed

## Review Notes

- implementation complete; reviewed against schema, code, and AGENTS.md rules
- second-pass review after fixes: previous P1/P2/P3 findings are resolved; one new low-priority
  consistency note recorded below

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] DB CHECK constraint on `password_credentials.password_algo` allows only the literal
  `argon2id` (see [000001_create_identity_tables.up.sql:42](services/identity/migrations/postgres/000001_create_identity_tables.up.sql:42)).
  This contradicts the acceptance criterion that algorithm and parameters are persisted with the
  hash so future upgrades are transparent — adding `argon2id-v2` or `bcrypt` will require an
  extra migration to widen the constraint. Either drop the CHECK or list the planned successor
  algorithms now.
  Resolution: changed the DB invariant to require a non-empty algorithm identifier without
  pinning the allowed set to only `argon2id`.

### P2 Medium

- [x] `accounts.email` (verbatim) and `accounts.normalized_email` (canonical) can drift if
  `email` is ever updated without re-normalizing; there is no DB-side invariant linking them
  (see [repository.go:255](services/identity/internal/repository/postgres/repository.go:255)
  and [auth_service.go:54-58](services/identity/internal/service/auth_service.go:54)). Use a
  generated column (`normalized_email GENERATED ALWAYS AS (lower(trim(email))) STORED`) or store
  only the canonical form alongside a separate display field.
  Resolution: `normalized_email` is now a generated stored column derived from `email`.
- [x] `oauth_states.pkce_verifier` is persisted in plaintext for the full 10-minute TTL
  (see [oauth.go:73-83](services/identity/internal/service/oauth.go:73)). The verifier acts as
  the PKCE secret on token exchange; a DB snapshot during the window combined with a captured
  `code` would let an attacker complete the exchange. Store only `verifier_hash` and keep the
  plaintext verifier in the short-lived `oauth_state_<provider>` cookie, or accept and document
  the residual risk.
  Resolution: identity now stores only `pkce_verifier_hash`; the plaintext verifier is kept in a
  short-lived provider PKCE cookie for the browser callback.
- [x] `sessions_token_hash_uq` (see
  [000001_create_identity_tables.up.sql:57](services/identity/migrations/postgres/000001_create_identity_tables.up.sql:57))
  is a global unique index; combined with the partial active-lookup index it permanently grows
  with every revoked session. Consider a partial unique on `WHERE revoked_at IS NULL` once a
  cleanup job is in scope.
  Resolution: replaced the global session token unique index with a partial active-session
  unique index on `WHERE revoked_at IS NULL`.

### P3 Low

- [x] No cleanup job is wired for expired/consumed `oauth_states` rows or revoked `sessions`;
  both grow unbounded. Acceptable for bootstrap but should be tracked.
  Resolution: tracked in follow-up `E14-006` rather than introducing background cleanup runtime in
  the identity bootstrap slice.
- [x] `auth_service.Signup` populates `account.NormalizedEmail` on the in-memory struct
  ([auth_service.go:54-62](services/identity/internal/service/auth_service.go:54)), but
  `insertAccount` no longer writes that column because it is generated stored
  ([repository.go:349-357](services/identity/internal/repository/postgres/repository.go:349)).
  The Go struct's pre-commit value can diverge from the DB-truth value if the trim/lower rules
  ever drift between Go (`strings.ToLower(strings.TrimSpace(...))`) and Postgres
  (`lower(btrim(...))`). Either drop the field assignment in `Signup` and rely on a post-insert
  re-read, or align the normalization helper one-to-one with the DB expression.
  Resolution: aligned `NormalizeEmail` with the DB expression by trimming only ASCII spaces before
  lowercasing, and added a focused repository test for the normalization contract.
