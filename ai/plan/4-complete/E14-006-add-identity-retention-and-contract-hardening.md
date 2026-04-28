# E14-006 - Add Identity Retention And Contract Hardening

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Add explicit identity retention cleanup and document the browser/session auth contracts that were
called out during E14 review.

## Scope

- add an explicit retention path for expired or consumed `oauth_states` rows
- add an explicit retention path for revoked or long-expired `sessions` rows
- evaluate the password salt storage shape before the credential format becomes externally
  depended on
- document the browser/session CSRF contract for `services/web` and the E15 gateway integration
- exclude new queues, schedulers, or background runtimes unless the implementation pass proves a
  simple local cleanup command is insufficient

## Acceptance Criteria

- identity storage has a documented and test-covered cleanup path for expired OAuth state rows
- identity storage has a documented and test-covered cleanup path for revoked or expired session
  rows
- credential storage format is either kept with a documented rationale or migrated to an explicit
  salt column
- session and CSRF cookie/header contracts explain which values are read from JSON bodies and
  which values are read from cookies
- local validation covers the cleanup and contract changes

## Assumptions

- E14 bootstrap should not introduce background jobs unless retention pressure becomes a functional
  requirement
- PostgreSQL remains the source of truth for identity state
- E15 gateway work will consume the documented session validation and CSRF contracts

## Implementation Notes

- prefer repository-level cleanup methods plus a local maintenance command before adding a
  continuously running worker
- keep cleanup behavior idempotent and bounded by explicit timestamps
- preserve the layered handler/service/repository boundaries if an HTTP or CLI trigger is added
- implemented `identity-maintenance cleanup` as a one-shot local maintenance command
- added repository cleanup methods for expired/consumed `oauth_states` and revoked/expired
  `sessions`
- added a `RetentionService` with explicit configured cutoffs:
  `IDENTITY_OAUTH_STATE_RETENTION_HOURS` and `IDENTITY_SESSION_RETENTION_DAYS`
- kept password salt in `password_params` for v1 and documented the rationale in
  `docs/identity_auth_service.md`
- documented session cookies, CSRF bootstrap/rotation, OAuth temporary cookies, cleanup behavior,
  and web integration in `docs/identity_auth_service.md`

## Progress Checklist

- [x] confirm cleanup trigger and retention windows
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `gofmt -w services/identity`
- `go test ./services/identity/internal/config ./services/identity/internal/service` passed
- `task test` passed
- `task lint` passed
- `task build` passed
- `git diff --check` passed

## Review Notes

- implementation complete; reviewed against retention service, repository cleanup queries, the
  `identity-maintenance` command, and `docs/identity_auth_service.md`

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] `DeleteOAuthStatesBefore` and `DeleteSessionsBefore` in
  [repository.go:225-249](services/identity/internal/repository/postgres/repository.go:225)
  issue unbounded `DELETE` statements with no `LIMIT` and no batching. For a healthy v1 database
  this is fine, but on tables with millions of revoked sessions a single `DELETE` can hold long
  table locks. Note this as a known limitation in the runbook or split into a `DELETE ... WHERE
  ... LIMIT` loop when the tables grow.
  Resolution: cleanup now deletes bounded batches via `ctid IN (SELECT ... LIMIT $2)`, and the
  service loops until a partial batch is returned.
- [x] `cmd/identity-maintenance/main.go:25` hardcodes a 30-second context timeout for the whole
  cleanup run. Combined with unbounded deletes, a large backlog will fail the run with a
  context-deadline error rather than completing. Make the timeout configurable (env var) or
  switch to bounded-batch deletion that re-checks the timeout each iteration.
  Resolution: maintenance timeout is now configurable via
  `IDENTITY_MAINTENANCE_TIMEOUT_SECONDS`, defaulting to `120`.
- [x] Retention coverage relies on a fake repository in
  [retention_test.go](services/identity/internal/service/retention_test.go); the actual SQL
  delete clauses (especially the `OR` between `expires_at` and `absolute_expires_at`) are not
  exercised against a real Postgres. Consider adding a single integration test backed by
  testcontainers or `task test:integration` so a regression in the SQL cannot pass CI.
  Resolution: service coverage now verifies bounded batch looping and limits. SQL integration
  coverage remains intentionally deferred until the repo has a real integration-test lane.
- [x] `docs/identity_auth_service.md` mentions the retention environment variables and defaults
  (24h / 30d) but does not explain the rationale for those numbers. One-line guidance — for
  example "OAuth state retention is short because consumed states have no audit value beyond the
  failed-callback diagnostic window; session retention matches the absolute session lifetime so
  forensic lookback covers a full session" — would help operators tune them.
  Resolution: documented the rationale for the default retention windows, plus the new batch size
  and timeout settings.
