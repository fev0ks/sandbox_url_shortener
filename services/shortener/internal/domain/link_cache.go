package domain

import (
	"context"
	"time"
)

// LinkCache is an optional read-side cache for the redirect hot path.
//
// Implementations MUST be tolerant of transient failures: returning an error
// from Get/Set/Delete must not break correctness of the redirect or write
// flows. Callers degrade to the authoritative repository on cache errors.
type LinkCache interface {
	// Get returns the cached link for the given short code, or (nil, nil) on miss.
	Get(ctx context.Context, shortCode string) (*CachedLink, error)

	// Set stores the link under the given short code with the given TTL.
	// A non-positive ttl means the entry must NOT be written.
	Set(ctx context.Context, shortCode string, link *CachedLink, ttl time.Duration) error

	// Delete removes the cached entry for the short code. Missing keys are not an error.
	Delete(ctx context.Context, shortCode string) error
}

// CachedLink is the minimal subset of a link needed to serve a redirect.
//
// We deliberately do NOT cache mutable counters that affect correctness:
// links with MaxClicks > 0 always confirm against PostgreSQL even on hit,
// because the cache cannot keep RedirectCount in sync under load.
type CachedLink struct {
	OriginalURL string     `json:"original_url"`
	MaxClicks   int64      `json:"max_clicks"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
