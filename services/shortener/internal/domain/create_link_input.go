package domain

import "time"

// CreateLinkInput holds validated business input for creating a new link.
type CreateLinkInput struct {
	OriginalURL string
	Title       *string
	ExpiresAt   *time.Time
	MaxClicks   int64
}
