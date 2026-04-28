// Package postgres implements identity repositories using PostgreSQL.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/services/identity/internal/domain"
)

// Repository implements identity persistence backed by PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a PostgreSQL-backed identity repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreatePasswordAccount inserts an account, its password auth identity, and password credential.
func (r *Repository) CreatePasswordAccount(ctx context.Context, account *domain.Account, credential *domain.PasswordCredential) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("identity repo begin password account: %w", err)
	}
	defer rollback(ctx, tx)

	if err := insertAccount(ctx, tx, account); err != nil {
		return err
	}
	identityID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("identity repo password identity uuid: %w", err)
	}
	const identityQ = `
		INSERT INTO auth_identities (id, account_id, provider, provider_subject, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, '{}'::jsonb, $5, $5)`
	if _, err := tx.Exec(ctx, identityQ, identityID, account.ID, domain.ProviderPassword, account.NormalizedEmail, account.CreatedAt); err != nil {
		return fmt.Errorf("identity repo insert password identity: %w", err)
	}
	//nolint:gosec // SQL table and column names are not hardcoded credentials.
	const credentialQ = `
		INSERT INTO password_credentials (account_id, password_hash, password_algo, password_params, updated_at)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := tx.Exec(ctx, credentialQ, credential.AccountID, credential.Hash, credential.Algorithm, credential.Params, credential.UpdatedAt); err != nil {
		return fmt.Errorf("identity repo insert password credential: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("identity repo commit password account: %w", err)
	}
	return nil
}

// GetPasswordAccountByEmail loads the active password account and credential by normalized email.
func (r *Repository) GetPasswordAccountByEmail(ctx context.Context, normalizedEmail string) (*domain.Account, *domain.PasswordCredential, error) {
	const q = `
		SELECT a.id, a.email, a.normalized_email, a.display_name, a.status, a.email_verified_at,
		       a.created_at, a.updated_at,
		       pc.account_id, pc.password_hash, pc.password_algo, pc.password_params, pc.updated_at
		FROM accounts a
		JOIN password_credentials pc ON pc.account_id = a.id
		WHERE a.normalized_email = $1
		  AND a.status <> $2`
	var account domain.Account
	var credential domain.PasswordCredential
	err := r.pool.QueryRow(ctx, q, normalizedEmail, domain.AccountStatusDeleted).Scan(
		&account.ID, &account.Email, &account.NormalizedEmail, &account.DisplayName, &account.Status, &account.EmailVerifiedAt,
		&account.CreatedAt, &account.UpdatedAt,
		&credential.AccountID, &credential.Hash, &credential.Algorithm, &credential.Params, &credential.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, domain.ErrUnauthorized
	}
	if err != nil {
		return nil, nil, fmt.Errorf("identity repo get password account: %w", err)
	}
	return &account, &credential, nil
}

// GetSessionByTokenHash loads a session and account by server-side token hash.
func (r *Repository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.SessionAccount, error) {
	const q = `
		SELECT s.id, s.account_id, s.token_hash, s.issued_at, s.expires_at, s.last_seen_at,
		       s.revoked_at, s.absolute_expires_at,
		       a.id, a.email, a.normalized_email, a.display_name, a.status, a.email_verified_at,
		       a.created_at, a.updated_at
		FROM sessions s
		JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1
		  AND a.status <> $2`
	var session domain.Session
	var account domain.Account
	err := r.pool.QueryRow(ctx, q, tokenHash, domain.AccountStatusDeleted).Scan(
		&session.ID, &session.AccountID, &session.TokenHash, &session.IssuedAt, &session.ExpiresAt, &session.LastSeenAt,
		&session.RevokedAt, &session.AbsoluteExp,
		&account.ID, &account.Email, &account.NormalizedEmail, &account.DisplayName, &account.Status, &account.EmailVerifiedAt,
		&account.CreatedAt, &account.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("identity repo get session: %w", err)
	}
	providers, err := r.ListAccountProviders(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	return &domain.SessionAccount{Account: &account, Session: &session, Providers: providers}, nil
}

// CreateSession persists a new session.
func (r *Repository) CreateSession(ctx context.Context, session *domain.Session) error {
	const q = `
		INSERT INTO sessions (
			id, account_id, token_hash, issued_at, expires_at, last_seen_at, revoked_at, absolute_expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := r.pool.Exec(ctx, q,
		session.ID, session.AccountID, session.TokenHash, session.IssuedAt, session.ExpiresAt,
		session.LastSeenAt, session.RevokedAt, session.AbsoluteExp,
	); err != nil {
		return fmt.Errorf("identity repo create session: %w", err)
	}
	return nil
}

// RevokeSessionByTokenHash marks a session revoked and reports whether an active row changed.
func (r *Repository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	if tokenHash == "" {
		return false, nil
	}
	const q = `UPDATE sessions SET revoked_at = $2 WHERE token_hash = $1 AND revoked_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, tokenHash, now)
	if err != nil {
		return false, fmt.Errorf("identity repo revoke session: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// TouchSession updates last-seen and sliding expiry data.
func (r *Repository) TouchSession(ctx context.Context, sessionID uuid.UUID, lastSeenAt, expiresAt time.Time) error {
	const q = `UPDATE sessions SET last_seen_at = $2, expires_at = $3 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, sessionID, lastSeenAt, expiresAt); err != nil {
		return fmt.Errorf("identity repo touch session: %w", err)
	}
	return nil
}

// ListAccountProviders returns auth provider names bound to an account.
func (r *Repository) ListAccountProviders(ctx context.Context, accountID uuid.UUID) ([]string, error) {
	const q = `
		SELECT provider
		FROM auth_identities
		WHERE account_id = $1
		ORDER BY provider`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity repo list providers: %w", err)
	}
	defer rows.Close()
	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, fmt.Errorf("identity repo scan provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity repo providers rows: %w", err)
	}
	return providers, nil
}

// CreateOAuthState persists a one-time OAuth round-trip state.
func (r *Repository) CreateOAuthState(ctx context.Context, state *domain.OAuthState) error {
	const q = `
		INSERT INTO oauth_states (
			id, provider, state_hash, pkce_verifier_hash, nonce_hash, redirect_url, expires_at, consumed_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := r.pool.Exec(ctx, q,
		state.ID, state.Provider, state.StateHash, state.PKCEVerifierHash, state.NonceHash,
		state.RedirectURL, state.ExpiresAt, state.ConsumedAt, state.CreatedAt,
	); err != nil {
		return fmt.Errorf("identity repo create oauth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState marks a matching state as consumed and returns it.
func (r *Repository) ConsumeOAuthState(ctx context.Context, provider, stateHash string, now time.Time) (*domain.OAuthState, error) {
	const q = `
		UPDATE oauth_states
		SET consumed_at = $3
		WHERE provider = $1 AND state_hash = $2 AND consumed_at IS NULL
		RETURNING id, provider, state_hash, pkce_verifier_hash, nonce_hash, redirect_url, expires_at, consumed_at, created_at`
	var state domain.OAuthState
	err := r.pool.QueryRow(ctx, q, provider, stateHash, now).Scan(
		&state.ID, &state.Provider, &state.StateHash, &state.PKCEVerifierHash, &state.NonceHash,
		&state.RedirectURL, &state.ExpiresAt, &state.ConsumedAt, &state.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOAuthStateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("identity repo consume oauth state: %w", err)
	}
	if !state.ExpiresAt.After(now) {
		return nil, domain.ErrOAuthStateExpired
	}
	return &state, nil
}

// DeleteOAuthStatesBefore removes one bounded batch of expired or consumed OAuth state rows.
func (r *Repository) DeleteOAuthStatesBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	const q = `
		DELETE FROM oauth_states
		WHERE ctid IN (
			SELECT ctid
			FROM oauth_states
			WHERE expires_at < $1
			   OR (consumed_at IS NOT NULL AND consumed_at < $1)
			LIMIT $2
		)`
	tag, err := r.pool.Exec(ctx, q, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("identity repo delete oauth states: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteSessionsBefore removes one bounded batch of revoked or expired session rows.
func (r *Repository) DeleteSessionsBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	const q = `
		DELETE FROM sessions
		WHERE ctid IN (
			SELECT ctid
			FROM sessions
			WHERE (revoked_at IS NOT NULL AND revoked_at < $1)
			   OR expires_at < $1
			   OR absolute_expires_at < $1
			LIMIT $2
		)`
	tag, err := r.pool.Exec(ctx, q, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("identity repo delete sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

// FindOrCreateOAuthAccount applies the E14 linking rule and binds a provider identity.
func (r *Repository) FindOrCreateOAuthAccount(ctx context.Context, claims domain.OAuthClaims, now time.Time) (*domain.Account, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("identity repo begin oauth account: %w", err)
	}
	defer rollback(ctx, tx)

	account, bound, err := r.oauthAccountInTx(ctx, tx, claims, now)
	if err != nil {
		return nil, err
	}
	if !bound {
		if err := insertAuthIdentity(ctx, tx, account.ID, claims, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("identity repo commit oauth account: %w", err)
	}
	return account, nil
}

func (r *Repository) oauthAccountInTx(ctx context.Context, tx pgx.Tx, claims domain.OAuthClaims, now time.Time) (*domain.Account, bool, error) {
	account, err := getAccountByProvider(ctx, tx, claims.Provider, claims.ProviderSubject)
	if err == nil {
		return account, true, nil
	}
	if !errors.Is(err, domain.ErrUnauthorized) {
		return nil, false, err
	}

	normalizedEmail := NormalizeEmail(claims.Email)
	existing, err := validateOAuthLinking(ctx, tx, claims, normalizedEmail)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}
	account, err = newOAuthAccount(claims, normalizedEmail, now)
	if err != nil {
		return nil, false, err
	}
	if err := insertAccount(ctx, tx, account); err != nil {
		return nil, false, err
	}
	return account, false, nil
}

func validateOAuthLinking(ctx context.Context, tx pgx.Tx, claims domain.OAuthClaims, normalizedEmail string) (*domain.Account, error) {
	if normalizedEmail == "" {
		return nil, domain.ErrOAuthLinkingBlocked
	}
	if !claims.EmailVerified {
		return nil, domain.ErrOAuthLinkingBlocked
	}
	existing, err := getAccountByEmail(ctx, tx, normalizedEmail)
	if err != nil && !errors.Is(err, domain.ErrUnauthorized) {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	if err := validateExistingOAuthLinking(claims, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func validateExistingOAuthLinking(claims domain.OAuthClaims, existing *domain.Account) error {
	if existing == nil {
		return nil
	}
	if claims.PrivateRelay {
		return domain.ErrOAuthLinkingBlocked
	}
	return nil
}

func newOAuthAccount(claims domain.OAuthClaims, normalizedEmail string, now time.Time) (*domain.Account, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("identity repo oauth account uuid: %w", err)
	}
	account := &domain.Account{
		ID:              id,
		Email:           claims.Email,
		NormalizedEmail: normalizedEmail,
		DisplayName:     optionalString(claims.DisplayName),
		Status:          domain.AccountStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if claims.EmailVerified {
		account.EmailVerifiedAt = &now
	}
	return account, nil
}

// NormalizeEmail returns the canonical email form used for uniqueness checks.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.Trim(email, " "))
}

func insertAccount(ctx context.Context, tx pgx.Tx, account *domain.Account) error {
	const q = `
		INSERT INTO accounts (
			id, email, display_name, status, email_verified_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.Exec(ctx, q,
		account.ID, account.Email, account.DisplayName, account.Status,
		account.EmailVerifiedAt, account.CreatedAt, account.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation {
			return domain.ErrAccountExists
		}
		return fmt.Errorf("identity repo insert account: %w", err)
	}
	return nil
}

func getAccountByProvider(ctx context.Context, tx pgx.Tx, provider, subject string) (*domain.Account, error) {
	const q = `
		SELECT a.id, a.email, a.normalized_email, a.display_name, a.status, a.email_verified_at,
		       a.created_at, a.updated_at
		FROM auth_identities ai
		JOIN accounts a ON a.id = ai.account_id
		WHERE ai.provider = $1 AND ai.provider_subject = $2 AND a.status <> $3`
	return scanAccount(tx.QueryRow(ctx, q, provider, subject, domain.AccountStatusDeleted))
}

func getAccountByEmail(ctx context.Context, tx pgx.Tx, normalizedEmail string) (*domain.Account, error) {
	const q = `
		SELECT id, email, normalized_email, display_name, status, email_verified_at, created_at, updated_at
		FROM accounts
		WHERE normalized_email = $1 AND status <> $2`
	return scanAccount(tx.QueryRow(ctx, q, normalizedEmail, domain.AccountStatusDeleted))
}

func insertAuthIdentity(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, claims domain.OAuthClaims, now time.Time) error {
	identityID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("identity repo auth identity uuid: %w", err)
	}
	metadata := claims.RawMetadata
	if len(metadata) == 0 {
		var err error
		metadata, err = json.Marshal(map[string]any{
			"email":          claims.Email,
			"email_verified": claims.EmailVerified,
			"private_relay":  claims.PrivateRelay,
		})
		if err != nil {
			return fmt.Errorf("identity repo auth identity metadata: %w", err)
		}
	}
	const q = `
		INSERT INTO auth_identities (id, account_id, provider, provider_subject, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $6)`
	_, err = tx.Exec(ctx, q, identityID, accountID, claims.Provider, claims.ProviderSubject, metadata, now)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation {
			return domain.ErrAccountExists
		}
		return fmt.Errorf("identity repo insert auth identity: %w", err)
	}
	return nil
}

func scanAccount(row pgx.Row) (*domain.Account, error) {
	var account domain.Account
	err := row.Scan(
		&account.ID, &account.Email, &account.NormalizedEmail, &account.DisplayName, &account.Status,
		&account.EmailVerifiedAt, &account.CreatedAt, &account.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUnauthorized
	}
	if err != nil {
		return nil, fmt.Errorf("identity repo scan account: %w", err)
	}
	return &account, nil
}

func rollback(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
