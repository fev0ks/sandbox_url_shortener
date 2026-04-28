package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/identity/internal/domain"
	identitypostgres "sandbox_url_shortener/services/identity/internal/repository/postgres"
)

func TestAuthServiceSignupAndLogin(t *testing.T) {
	repo := newFakeRepo()
	svc := NewAuthService(repo, slog.Default(), 30*24*time.Hour, 7*24*time.Hour)
	ctx := context.Background()
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

	signup, err := svc.Signup(ctx, domain.SignupInput{
		Email:       " USER@example.COM ",
		DisplayName: "User",
		Password:    "very-strong-password",
		Now:         now,
	}, "")
	if err != nil {
		t.Fatalf("Signup error: %v", err)
	}
	if signup.Token == "" {
		t.Fatal("expected session token")
	}
	if signup.Account.NormalizedEmail != "user@example.com" {
		t.Fatalf("normalized email = %q", signup.Account.NormalizedEmail)
	}
	if len(signup.Providers) != 1 || signup.Providers[0] != domain.ProviderPassword {
		t.Fatalf("providers = %#v", signup.Providers)
	}

	login, err := svc.Login(ctx, domain.LoginInput{
		Email:        "user@example.com",
		Password:     "very-strong-password",
		Now:          now.Add(time.Minute),
		CurrentToken: signup.Token,
	})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if login.Token == "" || login.Token == signup.Token {
		t.Fatal("expected rotated session token")
	}
	if !repo.sessionRevoked(signup.Token) {
		t.Fatal("expected prior session to be revoked")
	}
}

func TestAuthServiceRejectsBadPasswordAndExpiredSession(t *testing.T) {
	repo := newFakeRepo()
	svc := NewAuthService(repo, slog.Default(), 30*24*time.Hour, 7*24*time.Hour)
	ctx := context.Background()
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

	_, err := svc.Signup(ctx, domain.SignupInput{
		Email:    "user@example.com",
		Password: "very-strong-password",
		Now:      now,
	}, "")
	if err != nil {
		t.Fatalf("Signup error: %v", err)
	}

	if _, err := svc.Login(ctx, domain.LoginInput{
		Email:    "user@example.com",
		Password: "wrong-password",
		Now:      now,
	}); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("bad password err = %v", err)
	}

	login, err := svc.Login(ctx, domain.LoginInput{
		Email:    "user@example.com",
		Password: "very-strong-password",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if _, err := svc.CurrentSession(ctx, login.Token, now.Add(31*24*time.Hour)); !errors.Is(err, domain.ErrSessionExpired) {
		t.Fatalf("expired session err = %v", err)
	}
}

func TestValidateCSRF(t *testing.T) {
	if err := ValidateCSRF("token", "token"); err != nil {
		t.Fatalf("ValidateCSRF accepted token: %v", err)
	}
	if !errors.Is(ValidateCSRF("token", "other"), domain.ErrInvalidCSRF) {
		t.Fatal("expected invalid csrf")
	}
}

type fakeRepo struct {
	accounts    map[string]*domain.Account
	credentials map[string]*domain.PasswordCredential
	sessions    map[string]*domain.Session
	providers   map[uuid.UUID][]string
	oauthState  *domain.OAuthState

	oauthCleanupCutoff    time.Time
	sessionCleanupCutoff  time.Time
	oauthCleanupDeleted   []int64
	sessionCleanupDeleted []int64
	oauthCleanupErr       error
	sessionCleanupErr     error
	oauthCleanupLimits    []int
	sessionCleanupLimits  []int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		accounts:    map[string]*domain.Account{},
		credentials: map[string]*domain.PasswordCredential{},
		sessions:    map[string]*domain.Session{},
		providers:   map[uuid.UUID][]string{},
	}
}

func (r *fakeRepo) CreatePasswordAccount(_ context.Context, account *domain.Account, credential *domain.PasswordCredential) error {
	if _, ok := r.accounts[account.NormalizedEmail]; ok {
		return domain.ErrAccountExists
	}
	r.accounts[account.NormalizedEmail] = account
	r.credentials[account.NormalizedEmail] = credential
	r.providers[account.ID] = []string{domain.ProviderPassword}
	return nil
}

func (r *fakeRepo) GetPasswordAccountByEmail(_ context.Context, normalizedEmail string) (*domain.Account, *domain.PasswordCredential, error) {
	account := r.accounts[normalizedEmail]
	if account == nil {
		return nil, nil, domain.ErrUnauthorized
	}
	return account, r.credentials[normalizedEmail], nil
}

func (r *fakeRepo) GetSessionByTokenHash(_ context.Context, tokenHash string) (*domain.SessionAccount, error) {
	session := r.sessions[tokenHash]
	if session == nil {
		return nil, domain.ErrSessionNotFound
	}
	var account *domain.Account
	for _, candidate := range r.accounts {
		if candidate.ID == session.AccountID {
			account = candidate
			break
		}
	}
	if account == nil {
		return nil, domain.ErrSessionNotFound
	}
	return &domain.SessionAccount{Account: account, Session: session, Providers: r.providers[account.ID]}, nil
}

func (r *fakeRepo) CreateSession(_ context.Context, session *domain.Session) error {
	r.sessions[session.TokenHash] = session
	return nil
}

func (r *fakeRepo) RevokeSessionByTokenHash(_ context.Context, tokenHash string, now time.Time) (bool, error) {
	if session := r.sessions[tokenHash]; session != nil {
		if session.RevokedAt != nil {
			return false, nil
		}
		session.RevokedAt = &now
		return true, nil
	}
	return false, nil
}

func (r *fakeRepo) TouchSession(_ context.Context, sessionID uuid.UUID, lastSeenAt, expiresAt time.Time) error {
	for _, session := range r.sessions {
		if session.ID == sessionID {
			session.LastSeenAt = lastSeenAt
			session.ExpiresAt = expiresAt
		}
	}
	return nil
}

func (r *fakeRepo) ListAccountProviders(_ context.Context, accountID uuid.UUID) ([]string, error) {
	return r.providers[accountID], nil
}

func (r *fakeRepo) CreateOAuthState(_ context.Context, state *domain.OAuthState) error {
	r.oauthState = state
	return nil
}

func (r *fakeRepo) ConsumeOAuthState(_ context.Context, provider, stateHash string, now time.Time) (*domain.OAuthState, error) {
	if r.oauthState == nil || r.oauthState.Provider != provider || r.oauthState.StateHash != stateHash {
		return nil, domain.ErrOAuthStateNotFound
	}
	if !r.oauthState.ExpiresAt.After(now) {
		return nil, domain.ErrOAuthStateExpired
	}
	r.oauthState.ConsumedAt = &now
	return r.oauthState, nil
}

func (r *fakeRepo) FindOrCreateOAuthAccount(_ context.Context, claims domain.OAuthClaims, now time.Time) (*domain.Account, error) {
	email := identitypostgres.NormalizeEmail(claims.Email)
	account := r.accounts[email]
	if account == nil {
		id, _ := uuid.NewV7()
		account = &domain.Account{
			ID:              id,
			Email:           claims.Email,
			NormalizedEmail: email,
			Status:          domain.AccountStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		r.accounts[email] = account
	}
	r.providers[account.ID] = append(r.providers[account.ID], claims.Provider)
	return account, nil
}

func (r *fakeRepo) DeleteOAuthStatesBefore(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	r.oauthCleanupCutoff = cutoff
	r.oauthCleanupLimits = append(r.oauthCleanupLimits, limit)
	if r.oauthCleanupErr != nil {
		return 0, r.oauthCleanupErr
	}
	if len(r.oauthCleanupDeleted) == 0 {
		return 0, nil
	}
	deleted := r.oauthCleanupDeleted[0]
	r.oauthCleanupDeleted = r.oauthCleanupDeleted[1:]
	return deleted, nil
}

func (r *fakeRepo) DeleteSessionsBefore(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	r.sessionCleanupCutoff = cutoff
	r.sessionCleanupLimits = append(r.sessionCleanupLimits, limit)
	if r.sessionCleanupErr != nil {
		return 0, r.sessionCleanupErr
	}
	if len(r.sessionCleanupDeleted) == 0 {
		return 0, nil
	}
	deleted := r.sessionCleanupDeleted[0]
	r.sessionCleanupDeleted = r.sessionCleanupDeleted[1:]
	return deleted, nil
}

func (r *fakeRepo) sessionRevoked(rawToken string) bool {
	session := r.sessions[tokenHash(rawToken)]
	return session != nil && session.RevokedAt != nil
}
