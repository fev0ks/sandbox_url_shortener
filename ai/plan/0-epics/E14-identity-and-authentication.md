# E14 - Identity And Authentication

Status: active

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
- session or token issuance and validation contracts
- basic account and profile data used by `web`, `gateway`, and admin access
- excludes teams, orgs, billing, and rich profile management

## Key Decisions

- keep `identity` as one service for now; do not split `auth` and `users`
- support password, Google, and Apple in the first wave
- keep domain logic inside `identity`; `gateway` only validates and propagates identity context
- include required web auth screens and tasks inside the epic

## Task Links

- `E14-001-bootstrap-identity-service-and-account-schema.md`
- `E14-002-add-password-authentication-and-session-contracts.md`
- `E14-003-add-google-oauth-login.md`
- `E14-004-add-apple-oauth-login.md`
- `E14-005-add-web-auth-flows-and-session-aware-navigation.md`

## Open Questions

- whether sessions are cookie-first, token-first, or dual-mode
- whether account email verification belongs to v1 or later
