// Package postgres implements domain repository interfaces using PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/services/shortener/internal/domain"
)

// LinkRepo implements domain.LinkRepository backed by PostgreSQL.
type LinkRepo struct {
	pool *pgxpool.Pool
}

// NewLinkRepo creates a new PostgreSQL-backed link repository.
func NewLinkRepo(pool *pgxpool.Pool) *LinkRepo {
	return &LinkRepo{pool: pool}
}

// Create inserts a new link row. Returns domain.ErrShortCodeCollision on unique
// violation of the short_code partial index.
func (r *LinkRepo) Create(ctx context.Context, link *domain.Link) error {
	start := time.Now()
	result := pgResultSuccess
	defer func() {
		metrics.ObservePostgresOperation(shortenerService, pgOpLinkCreate, result, time.Since(start))
	}()

	const q = `
		INSERT INTO links (
			id, short_code, original_url, title, created_by,
			created_at, updated_at, expires_at, max_clicks,
			redirect_count, disabled_at, deleted_at, last_accessed_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13
		)`

	_, err := r.pool.Exec(ctx, q,
		link.ID, link.ShortCode, link.OriginalURL, link.Title, link.CreatedBy,
		link.CreatedAt, link.UpdatedAt, link.ExpiresAt, link.MaxClicks,
		link.RedirectCount, link.DisabledAt, link.DeletedAt, link.LastAccessedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgErrCodeUniqueViolation {
			result = pgResultShortCodeCollision
			return domain.ErrShortCodeCollision
		}
		result = pgResultError
		return fmt.Errorf("link repo create: %w", err)
	}
	return nil
}

// ListRecent returns non-deleted links ordered by newest first.
func (r *LinkRepo) ListRecent(ctx context.Context, limit int) ([]*domain.Link, error) {
	start := time.Now()
	result := pgResultSuccess
	defer func() {
		metrics.ObservePostgresOperation(shortenerService, pgOpLinkListRecent, result, time.Since(start))
	}()

	const q = `
		SELECT id, short_code, original_url, title, created_by,
		       created_at, updated_at, expires_at, max_clicks,
		       redirect_count, disabled_at, deleted_at, last_accessed_at
		FROM links
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		result = pgResultError
		return nil, fmt.Errorf("link repo list recent: %w", err)
	}
	defer rows.Close()

	links := make([]*domain.Link, 0, limit)
	for rows.Next() {
		var link domain.Link
		if err := rows.Scan(
			&link.ID, &link.ShortCode, &link.OriginalURL, &link.Title, &link.CreatedBy,
			&link.CreatedAt, &link.UpdatedAt, &link.ExpiresAt, &link.MaxClicks,
			&link.RedirectCount, &link.DisabledAt, &link.DeletedAt, &link.LastAccessedAt,
		); err != nil {
			result = pgResultError
			return nil, fmt.Errorf("link repo list recent scan: %w", err)
		}
		links = append(links, &link)
	}
	if err := rows.Err(); err != nil {
		result = pgResultError
		return nil, fmt.Errorf("link repo list recent rows: %w", err)
	}

	return links, nil
}

// GetByID returns an active (non-deleted) link by its internal UUID.
func (r *LinkRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	start := time.Now()
	link, err := r.scanLink(r.pool.QueryRow(ctx, `
		SELECT id, short_code, original_url, title, created_by,
		       created_at, updated_at, expires_at, max_clicks,
		       redirect_count, disabled_at, deleted_at, last_accessed_at
		FROM links
		WHERE id = $1 AND deleted_at IS NULL`, id))
	metrics.ObservePostgresOperation(shortenerService, pgOpLinkGetByID, postgresResult(err), time.Since(start))
	return link, err
}

// Disable sets disabled_at = now() on an active link.
func (r *LinkRepo) Disable(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `
		UPDATE links
		SET disabled_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING short_code`
	return r.execReturningShortCode(ctx, pgOpLinkDisable, q, id)
}

// Enable clears disabled_at on an active link.
func (r *LinkRepo) Enable(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `
		UPDATE links
		SET disabled_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING short_code`
	return r.execReturningShortCode(ctx, pgOpLinkEnable, q, id)
}

// SoftDelete sets deleted_at = now() on an active link.
func (r *LinkRepo) SoftDelete(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `
		UPDATE links
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING short_code`
	return r.execReturningShortCode(ctx, pgOpLinkSoftDelete, q, id)
}

// execReturningShortCode runs an UPDATE ... RETURNING short_code statement and
// maps the no-rows case to domain.ErrNotFound.
func (r *LinkRepo) execReturningShortCode(ctx context.Context, operation, q string, id uuid.UUID) (string, error) {
	start := time.Now()
	var shortCode string
	err := r.pool.QueryRow(ctx, q, id).Scan(&shortCode)
	metrics.ObservePostgresOperation(shortenerService, operation, postgresResult(err), time.Since(start))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("link repo exec returning: %w", err)
	}
	return shortCode, nil
}

// ResolveAndIncrement atomically increments redirect_count and returns the link
// if it is eligible for redirect. Falls back to a read-only query to distinguish
// 404 from 410 when the update affects zero rows.
func (r *LinkRepo) ResolveAndIncrement(ctx context.Context, shortCode string) (*domain.Link, error) {
	start := time.Now()
	const updateQ = `
		UPDATE links
		SET redirect_count = redirect_count + 1,
		    last_accessed_at = now(),
		    updated_at = now()
		WHERE short_code = $1
		  AND deleted_at IS NULL
		  AND disabled_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		  AND (max_clicks = 0 OR redirect_count < max_clicks)
		RETURNING id, short_code, original_url, title, created_by,
		          created_at, updated_at, expires_at, max_clicks,
		          redirect_count, disabled_at, deleted_at, last_accessed_at`

	link, err := r.scanLink(r.pool.QueryRow(ctx, updateQ, shortCode))
	metrics.ObservePostgresOperation(shortenerService, pgOpLinkResolveAndIncrement, postgresResult(err), time.Since(start))
	if err == nil {
		return link, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Update matched zero rows — determine if the link exists but is ineligible.
	return r.classifyMiss(ctx, shortCode)
}

// TouchRedirect increments redirect_count for an unlimited link without
// fetching the full row back to the application. This keeps PostgreSQL as the
// authoritative redirect counter on cache hits while avoiding a read-back query.
func (r *LinkRepo) TouchRedirect(ctx context.Context, shortCode string) error {
	start := time.Now()
	const q = `
		UPDATE links
		SET redirect_count = redirect_count + 1,
		    last_accessed_at = now(),
		    updated_at = now()
		WHERE short_code = $1
		  AND deleted_at IS NULL
		  AND disabled_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		  AND max_clicks = 0
		RETURNING 1`

	var touched int
	err := r.pool.QueryRow(ctx, q, shortCode).Scan(&touched)
	metrics.ObservePostgresOperation(shortenerService, pgOpLinkTouchRedirect, postgresResult(err), time.Since(start))
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("link repo touch redirect: %w", err)
	}

	_, classifyErr := r.classifyMiss(ctx, shortCode)
	return classifyErr
}

// classifyMiss performs a read-only lookup on a short code to distinguish 404 from 410.
func (r *LinkRepo) classifyMiss(ctx context.Context, shortCode string) (*domain.Link, error) {
	start := time.Now()
	const q = `
		SELECT deleted_at, disabled_at, expires_at, max_clicks, redirect_count
		FROM links
		WHERE short_code = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var l domain.Link
	err := r.pool.QueryRow(ctx, q, shortCode).Scan(
		&l.DeletedAt, &l.DisabledAt, &l.ExpiresAt, &l.MaxClicks, &l.RedirectCount,
	)
	metrics.ObservePostgresOperation(shortenerService, pgOpLinkClassifyMiss, postgresResult(err), time.Since(start))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("link repo classify miss: %w", err)
	}

	// Deleted or disabled → 404.
	if l.DeletedAt != nil || l.DisabledAt != nil {
		return nil, domain.ErrNotFound
	}

	// Expired or max clicks reached → 410.
	return nil, domain.ErrLinkGone
}

// scanLink scans a single row into a domain.Link.
func (r *LinkRepo) scanLink(row pgx.Row) (*domain.Link, error) {
	var l domain.Link
	err := row.Scan(
		&l.ID, &l.ShortCode, &l.OriginalURL, &l.Title, &l.CreatedBy,
		&l.CreatedAt, &l.UpdatedAt, &l.ExpiresAt, &l.MaxClicks,
		&l.RedirectCount, &l.DisabledAt, &l.DeletedAt, &l.LastAccessedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("link repo scan: %w", err)
	}
	return &l, nil
}

func postgresResult(err error) string {
	switch {
	case err == nil:
		return pgResultSuccess
	case errors.Is(err, pgx.ErrNoRows), errors.Is(err, domain.ErrNotFound):
		return pgResultNotFound
	default:
		return pgResultError
	}
}
