package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository owns identity persistence.
type Repository interface {
	CreatePasswordAccount(ctx context.Context, account *Account, credential *PasswordCredential) error
	GetPasswordAccountByEmail(ctx context.Context, normalizedEmail string) (*Account, *PasswordCredential, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*SessionAccount, error)
	CreateSession(ctx context.Context, session *Session) error
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (bool, error)
	TouchSession(ctx context.Context, sessionID uuid.UUID, lastSeenAt, expiresAt time.Time) error
	ListAccountProviders(ctx context.Context, accountID uuid.UUID) ([]string, error)

	CreateOAuthState(ctx context.Context, state *OAuthState) error
	ConsumeOAuthState(ctx context.Context, provider, stateHash string, now time.Time) (*OAuthState, error)
	FindOrCreateOAuthAccount(ctx context.Context, claims OAuthClaims, now time.Time) (*Account, error)

	DeleteOAuthStatesBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	DeleteSessionsBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}
