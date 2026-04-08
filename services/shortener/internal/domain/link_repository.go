package domain

import (
	"context"

	"github.com/google/uuid"
)

// LinkRepository defines persistence operations for links.
type LinkRepository interface {
	// Create inserts a new link. Returns ErrShortCodeCollision if the short code
	// violates the unique partial index.
	Create(ctx context.Context, link *Link) error

	// GetByID returns an active (non-deleted) link by its internal ID.
	// Returns ErrNotFound if the link does not exist or is soft-deleted.
	GetByID(ctx context.Context, id uuid.UUID) (*Link, error)

	// Disable sets disabled_at on an active link and returns the affected short_code
	// so the caller can invalidate any cache entry. Returns ErrNotFound if the link
	// does not exist or is soft-deleted.
	Disable(ctx context.Context, id uuid.UUID) (string, error)

	// Enable clears disabled_at on an active link and returns the affected short_code.
	// Returns ErrNotFound if the link does not exist or is soft-deleted.
	Enable(ctx context.Context, id uuid.UUID) (string, error)

	// SoftDelete sets deleted_at on an active link and returns the affected short_code.
	// Returns ErrNotFound if the link does not exist or is already soft-deleted.
	SoftDelete(ctx context.Context, id uuid.UUID) (string, error)

	// ResolveAndIncrement atomically increments redirect_count and returns the link
	// if it is eligible for redirect. Returns ErrNotFound for unknown/disabled/deleted
	// links and ErrLinkGone for expired or max-clicks-exhausted links.
	ResolveAndIncrement(ctx context.Context, shortCode string) (*Link, error)

	// TouchRedirect increments redirect_count for an unlimited link without fetching
	// the full row. It is used by the cache-hit redirect path so PostgreSQL remains
	// the authoritative counter while the hot path avoids a read-back query.
	// Returns ErrNotFound for unknown/disabled/deleted links and ErrLinkGone for
	// expired links or links that are no longer eligible for redirect.
	TouchRedirect(ctx context.Context, shortCode string) error
}
