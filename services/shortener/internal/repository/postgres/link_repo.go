// Package postgres implements domain repository interfaces using PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

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
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrShortCodeCollision
		}
		return fmt.Errorf("link repo create: %w", err)
	}
	return nil
}

// GetByID returns an active (non-deleted) link by its internal UUID.
func (r *LinkRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	const q = `
		SELECT id, short_code, original_url, title, created_by,
		       created_at, updated_at, expires_at, max_clicks,
		       redirect_count, disabled_at, deleted_at, last_accessed_at
		FROM links
		WHERE id = $1 AND deleted_at IS NULL`

	return r.scanLink(r.pool.QueryRow(ctx, q, id))
}

// Disable sets disabled_at = now() on an active link.
func (r *LinkRepo) Disable(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE links
		SET disabled_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	return r.execExpectOne(ctx, q, id)
}

// Enable clears disabled_at on an active link.
func (r *LinkRepo) Enable(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE links
		SET disabled_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	return r.execExpectOne(ctx, q, id)
}

// SoftDelete sets deleted_at = now() on an active link.
func (r *LinkRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE links
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`

	return r.execExpectOne(ctx, q, id)
}

// ResolveAndIncrement atomically increments redirect_count and returns the link
// if it is eligible for redirect. Falls back to a read-only query to distinguish
// 404 from 410 when the update affects zero rows.
func (r *LinkRepo) ResolveAndIncrement(ctx context.Context, shortCode string) (*domain.Link, error) {
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
	if err == nil {
		return link, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	// Update matched zero rows — determine if the link exists but is ineligible.
	return r.classifyMiss(ctx, shortCode)
}

// classifyMiss performs a read-only lookup on a short code to distinguish 404 from 410.
func (r *LinkRepo) classifyMiss(ctx context.Context, shortCode string) (*domain.Link, error) {
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

// execExpectOne runs a statement and returns ErrNotFound when zero rows are affected.
func (r *LinkRepo) execExpectOne(ctx context.Context, q string, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("link repo exec: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
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
