// Package handler provides HTTP handlers for the URL shortener API.
package handler

import (
	"time"

	"sandbox_url_shortener/internal/domain"
)

// CreateLinkRequest is the JSON body for POST /api/v1/links.
type CreateLinkRequest struct {
	OriginalURL string     `json:"original_url"`
	Title       *string    `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	MaxClicks   *int64     `json:"max_clicks,omitempty"`
}

// LinkResponse is the JSON representation of a link returned by the API.
type LinkResponse struct {
	ID             string     `json:"id"`
	ShortCode      string     `json:"short_code"`
	ShortURL       string     `json:"short_url"`
	OriginalURL    string     `json:"original_url"`
	Title          *string    `json:"title,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	MaxClicks      int64      `json:"max_clicks"`
	RedirectCount  int64      `json:"redirect_count"`
	DisabledAt     *time.Time `json:"disabled_at,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
}

// toLinkResponse converts a domain.Link to a LinkResponse.
func toLinkResponse(link *domain.Link, baseURL string) LinkResponse {
	return LinkResponse{
		ID:             link.ID.String(),
		ShortCode:      link.ShortCode,
		ShortURL:       baseURL + "/" + link.ShortCode,
		OriginalURL:    link.OriginalURL,
		Title:          link.Title,
		CreatedAt:      link.CreatedAt,
		UpdatedAt:      link.UpdatedAt,
		ExpiresAt:      link.ExpiresAt,
		MaxClicks:      link.MaxClicks,
		RedirectCount:  link.RedirectCount,
		DisabledAt:     link.DisabledAt,
		LastAccessedAt: link.LastAccessedAt,
	}
}
