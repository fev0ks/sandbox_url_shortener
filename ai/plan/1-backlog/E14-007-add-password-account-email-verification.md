# E14-007 - Add Password Account Email Verification

Status: backlog

Epic: E14 - Identity And Authentication

## Summary

Add a first-party email verification flow for password-created accounts so the identity service can
mark `accounts.email_verified_at` without manual SQL and later tighten OAuth account-linking rules.

## Scope

- issue single-use verification tokens for password-created accounts
- store token hashes, expiry, consumption timestamp, and request metadata in identity-owned
  PostgreSQL tables
- add endpoints to start/resend verification and complete verification
- add web screens for "check your email" and verification result states
- set `accounts.email_verified_at = now()` after successful verification
- emit structured audit logs for verification requested, completed, expired, and rejected states
- document local/dev mail delivery behavior
- excludes production email provider integration unless a concrete provider is selected
- excludes password reset and email change flows

## Acceptance Criteria

- password signup can trigger an email verification request without exposing raw tokens in logs
- verification completion marks the account verified exactly once
- expired, consumed, malformed, or wrong-account tokens fail closed with stable error codes
- resend is rate-limited or otherwise abuse-bounded for the local MVP
- OAuth linking docs identify whether `email_verified_at` is required after this task lands
- web UI exposes clear pending, success, expired, and failed verification states
- unit tests cover token creation, token consumption, expiry, replay, and repository failures

## Assumptions

- `accounts.email_verified_at` remains the canonical first-party verification marker
- token plaintext is only sent to the user-facing verification URL and never persisted
- local development can use a log/mailpit-style delivery adapter before real email provider setup
- current E14 OAuth linking allows provider-verified emails to link to existing password accounts
  until this flow is implemented and the policy is revisited

## Implementation Notes

- add `email_verification_tokens` under `services/identity/migrations/postgres`
- keep verification orchestration in the identity service layer
- keep delivery behind a small interface so local logging/mailpit and future provider delivery are
  swappable without changing handlers
- use existing request metadata and structured logger conventions
- consider adding a follow-up policy switch to require `email_verified_at` before OAuth linking

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] design storage and token contract
- [ ] implement verification start/resend
- [ ] implement verification completion
- [ ] add web verification states
- [ ] add tests
- [ ] update docs
- [ ] run validation
- [ ] pass review or resolve review findings

## Validation

- `go test ./services/identity/...`
- `cd services/web && npm test`
- `task web-build`
- manual local verification flow through the web app

## Review Notes

- pending

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
