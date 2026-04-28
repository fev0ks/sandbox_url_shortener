package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	AccountStatusActive   = "active"
	AccountStatusDisabled = "disabled"
	AccountStatusDeleted  = "deleted"

	ProviderPassword = "password"
	ProviderGoogle   = "google"
	ProviderApple    = "apple"

	PasswordAlgorithmArgon2ID = "argon2id"
)

// Account is the stable internal user record owned by identity.
type Account struct {
	ID              uuid.UUID
	Email           string
	NormalizedEmail string
	DisplayName     *string
	Status          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AuthIdentity binds an account to one authentication provider identity.
type AuthIdentity struct {
	ID              uuid.UUID
	AccountID       uuid.UUID
	Provider        string
	ProviderSubject string
	Metadata        []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PasswordCredential stores password material separately from account profile data.
type PasswordCredential struct {
	AccountID uuid.UUID
	Hash      string
	Algorithm string
	Params    []byte
	UpdatedAt time.Time
}

// Session is an identity-owned opaque browser session.
type Session struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	TokenHash   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	LastSeenAt  time.Time
	RevokedAt   *time.Time
	AbsoluteExp time.Time
}

// OAuthState stores one pending OAuth round trip.
type OAuthState struct {
	ID               uuid.UUID
	Provider         string
	StateHash        string
	PKCEVerifierHash string
	NonceHash        string
	RedirectURL      string
	ExpiresAt        time.Time
	ConsumedAt       *time.Time
	CreatedAt        time.Time
}

// SessionAccount is the transport-safe current-session view.
type SessionAccount struct {
	Account   *Account
	Session   *Session
	Providers []string
}

// SignupInput carries password signup data.
type SignupInput struct {
	Email       string
	DisplayName string
	Password    string
	Now         time.Time
	ClientIP    string
	UserAgent   string
}

// LoginInput carries password login data.
type LoginInput struct {
	Email        string
	Password     string
	Now          time.Time
	CurrentToken string
	ClientIP     string
	UserAgent    string
}

// OAuthClaims are normalized identity claims from an external OAuth provider.
type OAuthClaims struct {
	Provider        string
	ProviderSubject string
	Email           string
	EmailVerified   bool
	DisplayName     string
	PrivateRelay    bool
	RawMetadata     []byte
}

// OAuthLoginInput carries a verified provider identity into the account-linking flow.
type OAuthLoginInput struct {
	Claims       OAuthClaims
	Now          time.Time
	CurrentToken string
	ClientIP     string
	UserAgent    string
}
