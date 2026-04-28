package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/identity/internal/domain"
	identitypostgres "sandbox_url_shortener/services/identity/internal/repository/postgres"
)

// AuthService implements account, password, session, and OAuth account flows.
type AuthService struct {
	repo             domain.Repository
	log              *slog.Logger
	absoluteLifetime time.Duration
	idleTimeout      time.Duration
}

// NewAuthService creates an identity auth service.
func NewAuthService(repo domain.Repository, log *slog.Logger, absoluteLifetime, idleTimeout time.Duration) *AuthService {
	return &AuthService{repo: repo, log: log, absoluteLifetime: absoluteLifetime, idleTimeout: idleTimeout}
}

// IssuedSession carries a newly issued opaque token plus the persisted session view.
type IssuedSession struct {
	Token     string
	Session   *domain.Session
	Account   *domain.Account
	Providers []string
}

// Signup creates a password account and issues a fresh session.
func (s *AuthService) Signup(ctx context.Context, input domain.SignupInput, currentToken string) (*IssuedSession, error) {
	email := strings.TrimSpace(input.Email)
	normalizedEmail := identitypostgres.NormalizeEmail(email)
	if normalizedEmail == "" || !strings.Contains(normalizedEmail, "@") {
		return nil, fmt.Errorf("%w: email is invalid", domain.ErrInvalidInput)
	}
	if err := validatePasswordPolicy(input.Password); err != nil {
		return nil, err
	}
	now := normalizedNow(input.Now)
	accountID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("identity signup uuid: %w", err)
	}
	displayName := optionalString(strings.TrimSpace(input.DisplayName))
	account := &domain.Account{
		ID:              accountID,
		Email:           email,
		NormalizedEmail: normalizedEmail,
		DisplayName:     displayName,
		Status:          domain.AccountStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	hash, params, err := hashPassword(input.Password)
	if err != nil {
		return nil, err
	}
	credential := &domain.PasswordCredential{
		AccountID: accountID,
		Hash:      hash,
		Algorithm: domain.PasswordAlgorithmArgon2ID,
		Params:    params,
		UpdatedAt: now,
	}
	if err := s.repo.CreatePasswordAccount(ctx, account, credential); err != nil {
		if errors.Is(err, domain.ErrAccountExists) {
			return nil, err
		}
		return nil, fmt.Errorf("identity signup: %w", err)
	}
	issued, err := s.issueSession(ctx, account, now, currentToken)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, AuditEventSignup, account.ID.String(), normalizedEmail, input.ClientIP, input.UserAgent, AuditReasonSuccess)
	return issued, nil
}

// Login verifies email/password credentials and issues a rotated session.
func (s *AuthService) Login(ctx context.Context, input domain.LoginInput) (*IssuedSession, error) {
	now := normalizedNow(input.Now)
	normalizedEmail := identitypostgres.NormalizeEmail(input.Email)
	account, credential, err := s.repo.GetPasswordAccountByEmail(ctx, normalizedEmail)
	if err != nil {
		runDummyPasswordHash(input.Password)
		s.auditLogin(ctx, AuditEventLoginFailure, accountIDString(account), input, AuditReasonUnknownAccount)
		return nil, domain.ErrUnauthorized
	}
	if account.Status != domain.AccountStatusActive {
		s.auditLogin(ctx, AuditEventLoginFailure, account.ID.String(), input, AuditReasonAccountNotActive)
		return nil, domain.ErrAccountDisabled
	}
	ok, err := verifyPassword(input.Password, credential)
	if err != nil {
		return nil, fmt.Errorf("identity login verify: %w", err)
	}
	if !ok {
		s.auditLogin(ctx, AuditEventLoginFailure, account.ID.String(), input, AuditReasonBadPassword)
		return nil, domain.ErrUnauthorized
	}
	issued, err := s.issueSession(ctx, account, now, input.CurrentToken)
	if err != nil {
		return nil, err
	}
	s.auditLogin(ctx, AuditEventLoginSuccess, account.ID.String(), input, AuditReasonSuccess)
	return issued, nil
}

// CurrentSession validates a session token and returns a transport-safe view.
func (s *AuthService) CurrentSession(ctx context.Context, rawToken string, now time.Time) (*domain.SessionAccount, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, domain.ErrSessionNotFound
	}
	sessionAccount, err := s.repo.GetSessionByTokenHash(ctx, tokenHash(rawToken))
	if err != nil {
		return nil, err
	}
	if sessionAccount.Account.Status != domain.AccountStatusActive {
		return nil, domain.ErrAccountDisabled
	}
	now = normalizedNow(now)
	session := sessionAccount.Session
	if session.RevokedAt != nil {
		return nil, domain.ErrSessionRevoked
	}
	if !session.ExpiresAt.After(now) || !session.AbsoluteExp.After(now) {
		return nil, domain.ErrSessionExpired
	}
	nextExpires := now.Add(s.idleTimeout)
	if nextExpires.After(session.AbsoluteExp) {
		nextExpires = session.AbsoluteExp
	}
	if nextExpires.After(session.ExpiresAt) || now.Sub(session.LastSeenAt) > time.Hour {
		if err := s.repo.TouchSession(ctx, session.ID, now, nextExpires); err != nil {
			return nil, err
		}
		session.LastSeenAt = now
		session.ExpiresAt = nextExpires
	}
	return sessionAccount, nil
}

// Logout revokes the current session. It is safe for missing tokens.
func (s *AuthService) Logout(ctx context.Context, rawToken string, now time.Time, clientIP, userAgent string) error {
	if rawToken == "" {
		s.auditEvent(ctx, AuditEventLogout, "", "", clientIP, userAgent, AuditReasonMissingSession)
		return nil
	}
	revoked, err := s.repo.RevokeSessionByTokenHash(ctx, tokenHash(rawToken), normalizedNow(now))
	if err != nil {
		return err
	}
	if revoked {
		s.auditEvent(ctx, AuditEventLogout, "", "", clientIP, userAgent, AuditReasonRevoked)
		return nil
	}
	s.auditEvent(ctx, AuditEventLogout, "", "", clientIP, userAgent, AuditReasonSessionNotFound)
	return nil
}

// IssueOAuthSession applies account linking and issues a session after provider verification.
func (s *AuthService) IssueOAuthSession(ctx context.Context, input domain.OAuthLoginInput) (*IssuedSession, error) {
	now := normalizedNow(input.Now)
	account, err := s.repo.FindOrCreateOAuthAccount(ctx, input.Claims, now)
	if err != nil {
		return nil, err
	}
	issued, err := s.issueSession(ctx, account, now, input.CurrentToken)
	if err != nil {
		return nil, err
	}
	s.auditEvent(ctx, AuditEventLoginSuccess, account.ID.String(), account.NormalizedEmail, input.ClientIP, input.UserAgent, input.Claims.Provider)
	return issued, nil
}

func (s *AuthService) issueSession(ctx context.Context, account *domain.Account, now time.Time, currentToken string) (*IssuedSession, error) {
	if currentToken != "" {
		revoked, err := s.repo.RevokeSessionByTokenHash(ctx, tokenHash(currentToken), now)
		if err != nil {
			return nil, err
		}
		if revoked {
			s.auditEvent(ctx, AuditEventSessionRevocation, account.ID.String(), account.NormalizedEmail, "", "", AuditReasonRotation)
		}
	}
	rawToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("identity session uuid: %w", err)
	}
	session := &domain.Session{
		ID:          sessionID,
		AccountID:   account.ID,
		TokenHash:   tokenHash(rawToken),
		IssuedAt:    now,
		ExpiresAt:   now.Add(s.idleTimeout),
		LastSeenAt:  now,
		AbsoluteExp: now.Add(s.absoluteLifetime),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	s.log.InfoContext(ctx, "identity session issued",
		slog.String("operation", OperationAuthSessionIssue),
		slog.String("account_id", account.ID.String()),
		slog.String("session_id", session.ID.String()),
	)
	providers, err := s.repo.ListAccountProviders(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	return &IssuedSession{Token: rawToken, Session: session, Account: account, Providers: providers}, nil
}

func (s *AuthService) auditLogin(ctx context.Context, event, accountID string, input domain.LoginInput, reason string) {
	s.auditEvent(ctx, event, accountID, input.Email, input.ClientIP, input.UserAgent, reason)
}

func (s *AuthService) auditEvent(ctx context.Context, event, accountID, email, clientIP, userAgent, reason string) {
	s.log.InfoContext(ctx, "identity audit",
		slog.String("operation", OperationAuthAudit),
		slog.String("event", event),
		slog.String("account_id", accountID),
		slog.String("normalized_email_hash", tokenHash(identitypostgres.NormalizeEmail(email))),
		slog.String("client_ip", clientIP),
		slog.String("user_agent", userAgent),
		slog.String("reason_code", reason),
	)
}

func ValidateCSRF(cookieValue, headerValue string) error {
	if cookieValue == "" || headerValue == "" || !constantStringEqual(cookieValue, headerValue) {
		return domain.ErrInvalidCSRF
	}
	return nil
}

func NewCSRFToken() (string, error) {
	return randomToken()
}

func normalizedNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func accountIDString(account *domain.Account) string {
	if account == nil {
		return ""
	}
	return account.ID.String()
}
