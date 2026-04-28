# E14 - Identity And Authentication

Status: complete

## Summary

Identity epic for adding first-class accounts, password login, and OAuth-based sign-in to the
platform.

## Goal

Introduce a single `identity` service that owns account lifecycle, authentication methods,
session issuance, and the minimum profile data needed by the rest of the system.

## Scope

- `services/identity` runtime and storage
- email and password authentication
- Google OAuth and Apple OAuth
- opaque session issuance, validation, and revocation contracts
- CSRF protection contract for cookie-bound endpoints
- structured audit logging for authentication events (login success, failure, logout, session
  revocation)
- basic account and profile data used by `web`, `gateway`, and admin access
- excludes teams, orgs, billing, and rich profile management
- excludes account email verification flow; schema may carry `email_verified_at` placeholder but
  authentication does not depend on it in v1

## Key Decisions

- keep `identity` as one service for now; do not split `auth` and `users`
- support password, Google, and Apple in the first wave
- keep domain logic inside `identity`; `gateway` only validates and propagates identity context
- include required web auth screens and tasks inside the epic
- sessions are cookie-first and opaque (random server-issued ids stored in PostgreSQL); JWT is
  explicitly out of scope for v1
- session cookies are `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/` and rotated on every successful
  authentication to mitigate session fixation
- CSRF defense for state-changing cookie endpoints uses a double-submit token issued by identity
  and read from a non-`HttpOnly` companion cookie; `SameSite=Lax` is not relied on alone
- default account-linking rule across providers: link a successful OAuth login to an existing
  account only if the provider returns a verified email that matches the existing account's
  primary email; otherwise create a new account. Linking is never silent on unverified email
- `services/web` calls `identity` directly until E15 gateway is available; migration to gateway is
  owned by `E15-003`, not by E14 tasks
- E14 does not depend on E15 shipping; gateway integration is additive

## Task Links

- `E14-001-bootstrap-identity-service-and-account-schema.md`
- `E14-002-add-password-authentication-and-session-contracts.md`
- `E14-003-add-google-oauth-login.md`
- `E14-004-add-apple-oauth-login.md`
- `E14-005-add-web-auth-flows-and-session-aware-navigation.md`
- `E14-006-add-identity-retention-and-contract-hardening.md`
- follow-up backlog: `E14-007-add-password-account-email-verification.md`

## Open Questions

- resolved in `E14-002`: default session lifetime is 30-day absolute and 7-day idle, configurable
  through identity environment variables
- resolved for v1: login throttling is deferred to a future shared abuse-control layer; identity
  emits stable login failure audit fields now
- deferred to backlog `E14-007`: first-party password account email verification
