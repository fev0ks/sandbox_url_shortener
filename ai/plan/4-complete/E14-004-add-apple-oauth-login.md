# E14-004 - Add Apple OAuth Login

Status: complete

Epic: E14 - Identity And Authentication

## Summary

Add Sign in with Apple to identity so Apple provider identities can create or attach to internal
accounts and receive the shared session contract.

## Scope

- add Apple OAuth configuration, login initiation, `form_post` callback handling, state
  validation, ES256 client_secret JWT generation, and token exchange
- verify Apple identity claims, including `iss`, `aud`, `sub`, `exp`, `nonce`, and email
  availability expectations
- support first login account creation and repeat login for an existing Apple provider subject
- apply the epic-default account-linking rule with Apple-specific constraints: link only when
  Apple returns `email_verified=true` and the email is not a private-relay address; private-relay
  or missing-email cases must never silently merge with an existing account
- persist the email and name fields delivered only on the first authorization response so repeat
  logins can rely on stored data rather than provider re-emission
- issue the same identity-owned session as password and Google login, including session id
  rotation on every successful sign-in
- exclude Google OAuth changes and web page implementation

## Acceptance Criteria

- Apple OAuth `state` and `nonce` are generated, stored, validated, expire, and are single-use
- `form_post` callback parses POST form parameters and rejects GET callbacks
- client_secret JWT is generated per request using ES256, the configured Apple key id, team id,
  and PEM-encoded private key; key id is configurable to support rotation without code changes
- callback handling rejects invalid state, missing or mismatched nonce, provider errors, token
  exchange failures, issuer mismatch, audience mismatch, missing subject, expired claims, and
  unusable account-linking states
- successful Apple login creates or finds exactly one internal account and provider identity
- repeat Apple login works even when Apple does not resend the user's email or name
- private-relay or missing-email handling is explicit and does not merge unrelated accounts
- service and handler tests cover happy path, repeat login without email, private-relay email,
  invalid state, missing nonce, invalid client_secret, and account-linking edge cases

## Assumptions

- implementation must confirm current Sign in with Apple requirements from official Apple
  documentation before coding
- Apple provider subject plus provider name is the durable identity key; email is treated as
  mutable metadata and never as the primary identity key
- session issuance reuses the contract from `E14-002`
- account-linking is fixed by the epic default and additionally fails closed for private-relay or
  missing email
- Apple key material (private key PEM, key id, team id, services bundle id) is supplied through
  the same secrets plumbing as other provider config and never read from logs or echoed in errors

## Implementation Notes

- isolate Apple-specific client secret generation, token exchange, and claim verification behind a
  provider boundary
- keep Apple private key material in config or secrets plumbing only; never log it
- do not depend on email being present after the first Apple authorization response
- declare provider names, OAuth state keys, and callback contract strings as constants
- keep the implementation parallel to Google where useful, but avoid a heavy generic OAuth
  framework until duplication proves costly
- implemented Apple authorization start and `form_post` callback endpoints with state, nonce,
  ES256 client secret generation, token exchange, first-response user metadata parsing, safe
  private-relay handling, and shared session issuance

## Progress Checklist

- [x] confirm provider contract, redirect URLs, scopes, client secret signing, and email behavior
- [x] add Apple OAuth config and provider client boundary
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
- official Apple Sign in with Apple REST token docs checked for token-generation/validation flow

## Review Notes

- implementation complete; reviewed against Apple form_post flow, client_secret signing, and
  account-linking rule
- second-pass review after fixes: previous P0/P1/P2/P3 findings are resolved; one new P1
  recorded below about cookie attribute consistency for the Apple PKCE cookie

## Review Findings

### P0 Critical

- [x] `AppleProvider.Exchange` in
  [oauth_providers.go:181-198](services/identity/internal/service/oauth_providers.go:181) falls
  back to `emailFromUser` (extracted from the unsigned `user` form field via
  `parseAppleUser`) when `claims.Email` is empty. Apple does NOT sign the `user` field — only
  the id_token's `email` claim is signed. Accepting `user.email` as the account email lets a
  caller forge an arbitrary email by POSTing a chosen `user` payload alongside a real Apple
  `code`. Even though the linking rule blocks linking when `EmailVerified=false`, a brand-new
  account can still be created carrying the forged email as its primary identifier. Use only
  the id_token-signed `email`; treat `user.name` as display-only metadata and never derive
  `email` from `user`.
  Resolution: Apple now derives account email only from signed `id_token` claims; unsigned
  `user` data is parsed only for display-name metadata.

### P1 High

- [x] `oauthStateCookie` is set with `SameSite=Lax` (see
  [handler.go:334-345](services/identity/internal/transport/http/stdlib/auth/handler.go:334)),
  but Apple delivers the callback via cross-site `POST` (`response_mode=form_post`). `Lax`
  cookies are not sent on cross-site POSTs, so the cookie cross-check against the URL state in
  [handler.go:221-224](services/identity/internal/transport/http/stdlib/auth/handler.go:221) is
  effectively skipped for Apple. Either issue the OAuth state cookie with
  `SameSite=None; Secure` for Apple specifically, or remove the cookie correlation and rely
  solely on the DB-side single-use state — but document the choice so it does not regress.
  Resolution: Apple no longer receives an OAuth state correlation cookie; the callback relies on
  DB-side single-use state plus the separate short-lived PKCE verifier cookie, which is compatible
  with `form_post`.
- [x] The Apple PKCE cookie issued in
  [handler.go:359-374](services/identity/internal/transport/http/stdlib/auth/handler.go:359)
  uses `SameSite=None` but `Secure: h.cookieSecure`. Browsers reject any
  `SameSite=None` cookie that is not also `Secure` (RFC 6265bis), so when
  `IDENTITY_COOKIE_SECURE=false` the Apple flow silently breaks because no PKCE cookie returns
  on the cross-site POST. Either force `Secure=true` on the Apple PKCE cookie regardless of
  config (Apple OAuth requires HTTPS anyway), or fail-fast at startup when Apple is configured
  alongside `IDENTITY_COOKIE_SECURE=false` so the misconfiguration is loud.
  Resolution: config validation now fails fast when Apple OAuth is fully configured with
  `IDENTITY_COOKIE_SECURE=false`, and a config test covers the misconfiguration.

### P2 Medium

- [x] `verifyCommonClaims` for Apple uses `time.Now()` with no clock-skew tolerance on `exp`,
  same as Google (see
  [oauth_providers.go:178](services/identity/internal/service/oauth_providers.go:178)). Add a
  60–120s leeway.
  Resolution: Apple uses the same common claim validation leeway as Google.
- [x] `parseAppleUser` always trims a `firstName + " " + lastName` even when both are empty,
  yielding an empty string used downstream. Functionally fine, but explicit handling makes the
  intent clearer.
  Resolution: Apple user parsing now explicitly returns only non-empty display-name metadata.

### P3 Low

- [x] Apple `client_secret` JWT (see
  [oauth_providers.go:201-230](services/identity/internal/service/oauth_providers.go:201)) is
  regenerated on every token exchange. Apple allows up to 6 months of validity; cache a signed
  client secret for a few minutes to avoid per-request ECDSA work.
  Resolution: signed Apple client secrets are now cached briefly with an expiry guard.
- [x] `parseECPrivateKey` in
  [oauth_providers.go:232-246](services/identity/internal/service/oauth_providers.go:232)
  replaces literal `\n` with newlines to handle env-var encodings. Document this in config so
  operators do not double-escape the PEM.
  Resolution: `.env.example` now documents literal `\n` escaping for one-line PEM environment
  values.
