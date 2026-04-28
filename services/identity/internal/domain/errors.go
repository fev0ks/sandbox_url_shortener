// Package domain contains identity business types and contracts.
package domain

import "errors"

var (
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrAccountExists        = errors.New("account already exists")
	ErrAccountDisabled      = errors.New("account disabled")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("session expired")
	ErrSessionRevoked       = errors.New("session revoked")
	ErrInvalidCSRF          = errors.New("invalid csrf token")
	ErrOAuthStateNotFound   = errors.New("oauth state not found")
	ErrOAuthStateExpired    = errors.New("oauth state expired")
	ErrOAuthStateConsumed   = errors.New("oauth state consumed")
	ErrOAuthProviderInvalid = errors.New("oauth provider invalid")
	ErrOAuthUnconfigured    = errors.New("oauth provider unconfigured")
	ErrOAuthLinkingBlocked  = errors.New("oauth account linking blocked")
)
