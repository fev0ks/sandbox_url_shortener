package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// LinkService defines business operations for links.
type LinkService interface {
	// Create generates a short code, persists the link, and returns it.
	Create(ctx context.Context, input CreateLinkInput) (*Link, error)

	// GetByID returns an active link by its internal ID.
	GetByID(ctx context.Context, id uuid.UUID) (*Link, error)

	// Disable marks a link as disabled.
	Disable(ctx context.Context, id uuid.UUID) (*Link, error)

	// Enable re-enables a previously disabled link.
	Enable(ctx context.Context, id uuid.UUID) (*Link, error)

	// SoftDelete marks a link as deleted.
	SoftDelete(ctx context.Context, id uuid.UUID) error

	// Resolve resolves a short code to the original URL, atomically incrementing
	// redirect_count. Returns ErrNotFound or ErrLinkGone as appropriate.
	Resolve(ctx context.Context, shortCode string) (*Link, error)
}

// CreateLinkInput holds validated parameters for creating a new link.
type CreateLinkInput struct {
	OriginalURL string
	Title       *string
	ExpiresAt   *time.Time
	MaxClicks   int64
}
