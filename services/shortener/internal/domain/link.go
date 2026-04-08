// Package domain defines core business types and interfaces.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Link represents a shortened URL with its metadata and lifecycle timestamps.
type Link struct {
	ID             uuid.UUID
	ShortCode      string
	OriginalURL    string
	Title          *string
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ExpiresAt      *time.Time
	MaxClicks      int64
	RedirectCount  int64
	DisabledAt     *time.Time
	DeletedAt      *time.Time
	LastAccessedAt *time.Time
}
