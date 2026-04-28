# E14-003 - Add Google OAuth Login

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Add Google OAuth sign-in to identity so a Google account can create or attach to an internal
account and receive the same session contract used by password login.

## Scope

- add Google OAuth configuration, login initiation with PKCE and OIDC `nonce`, callback handling,
  state validation, and token exchange
- verify Google identity claims, including `iss`, `aud`, `exp`, `nonce`, `sub`, and
  `email_verified`
- map verified provider claims to the identity account and auth identity model
- support first login account creation and repeat login for an existing provider subject
- apply the epic-default account-linking rule: link to an existing account only when
  `email_verified=true` and the email matches the existing account's primary email; otherwise
  create a new account
- issue the same identity-owned session as password login after successful OAuth completion,
  including session id rotation on every successful sign-in
- exclude Apple OAuth and web page implementation

## Acceptance Criteria

- OAuth `state` and PKCE `code_verifier` are generated, stored, validated, expire, and are
  single-use; reuse is rejected
- callback handling rejects invalid state, missing or mismatched PKCE verifier, missing or
  mismatched OIDC `nonce`, provider errors, token exchange failures, issuer mismatch, audience
  mismatch, expired claims, missing subject, and unusable email states
- a Google account with `email_verified=false` is never linked to an existing internal account; it
  either logs into a previously-bound provider identity or creates a new account
- successful Google login creates or finds exactly one internal account and provider identity
- existing Google provider identities log in without creating duplicate accounts
- service and handler tests cover happy path, duplicate provider subject, verified-email link,
  unverified-email no-link, invalid state, missing PKCE verifier, missing/invalid nonce, and
  invalid provider claims

## Assumptions

- implementation must confirm current Google OAuth and OpenID Connect requirements from official
  Google documentation before coding
- Google login uses OpenID Connect identity claims; broader Google API access is out of scope
- OAuth temporary state (state, PKCE verifier, nonce) uses identity-owned storage with explicit
  TTL (default 10 minutes) and single-use semantics
- session issuance reuses the contract from `E14-002`, including the same cookie attributes,
  rotation rule, and CSRF token bootstrap
- the default account-linking rule is fixed by the epic; this task implements it rather than
  re-deciding it

## Implementation Notes

- keep provider-specific token exchange and claim verification behind a small interface so service
  tests can run without Google infrastructure
- treat provider subject plus provider name as the durable login identity, not mutable email alone
- do not log provider tokens, authorization codes, ID tokens, refresh tokens, or raw claims
- declare provider names, OAuth cookie or state keys, and callback contract strings as constants
- prefer a simple explicit account-linking rule over a generic provider abstraction in the first
  pass
- implemented Google authorization-code start and callback endpoints with stored state, PKCE
  verifier, OIDC nonce, token exchange, claim validation, safe account-linking, and shared session
  issuance
- provider configuration is optional; unconfigured environments return a stable
  `oauth_unconfigured` response

## Progress Checklist

- [x] confirm provider contract, redirect URLs, scopes, and account-linking rule
- [x] add Google OAuth config and provider client boundary
- [x] implement login initiation and state storage
- [x] implement callback verification and account/provider identity mapping
- [x] reuse session issuance after successful login
- [x] add unit and web client tests for success, edge, and failure paths
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go test ./...` passed
- `task lint` passed
- `go build ./services/identity/cmd/identity ./services/shortener/cmd/shortener ./services/analytics/cmd/analytics` passed
- `npm test` passed in `services/web`
- `npm run typecheck` passed in `services/web`
- `npm run build` passed in `services/web`
- official Google OIDC docs checked for state, nonce, auth request, and token exchange contract

## Review Notes

- implementation complete; reviewed against OAuth flow, repository linking, and acceptance
  criteria
- second-pass review after fixes: previous P2/P3 findings are resolved; one new P2 about
  link-target verification recorded below

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] `verifyCommonClaims` in
  [oauth.go:165-182](services/identity/internal/service/oauth.go:165) checks `claims.ExpiresAt
  <= now.Unix()` with no clock-skew tolerance. A small drift between identity and Google can
  reject otherwise-valid tokens at the boundary. Add a leeway of 60–120s on `exp` (and use
  `iat` if present).
  Resolution: common OIDC claim validation now applies a 120-second leeway on `exp` and rejects
  future `iat` values outside that leeway.
- [x] When Google returns `email_verified=false`, `validateOAuthLinking` correctly blocks
  linking to an existing account, but the fallthrough creates a brand-new account whose
  primary `email` is the unverified address (see
  [repository.go:276-311](services/identity/internal/repository/postgres/repository.go:276)).
  This silently introduces an unverified-email account; either reject the login outright when
  `email_verified=false`, or store the email under a clearly-unverified flag and refuse all
  future flows that assume primary-email ownership.
  Resolution: unverified provider emails are now rejected before creating a new primary-email
  account.
- [x] When Google returns `email_verified=true`, `oauthAccountInTx` binds a new
  `auth_identities` row to any matching account found by email, even if that existing account's
  own `email_verified_at IS NULL` (e.g., a password-only signup that never completed
  verification — see
  [repository.go:283-302](services/identity/internal/repository/postgres/repository.go:283)).
  The link decision can become stricter once a first-party email verification flow exists, but E14
  explicitly excludes that flow and the current local MVP should allow verified provider emails to
  attach to password-created accounts.
  Resolution: kept the provider-side `email_verified=true` requirement and private-relay block,
  removed the temporary target-account `email_verified_at IS NOT NULL` requirement, and created
  backlog task `E14-007-add-password-account-email-verification.md` for the first-party
  verification flow.

### P3 Low

- [x] `verifyIDTokenSignature` in
  [oauth_providers.go:317-346](services/identity/internal/service/oauth_providers.go:317)
  hardcodes `header.Alg != "RS256"`. Acceptable today, but make the accepted algorithm set
  explicit and easy to extend when Google adopts ES256.
  Resolution: accepted ID-token algorithms now come from an explicit set.
- [x] Interface parameter naming in
  [oauth.go:20-24](services/identity/internal/service/oauth.go:20) declares `nonce` while the
  callers and provider implementations pass the stored `nonce_hash`. Rename to `nonceHash` to
  match runtime semantics.
  Resolution: renamed the provider interface parameter to `nonceHash`.
