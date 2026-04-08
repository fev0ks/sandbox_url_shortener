package link

import (
	"time"

	"sandbox_url_shortener/services/shortener/internal/domain"
)

// CreateRequest is the JSON body for POST /api/v1/links.
type CreateRequest struct {
	OriginalURL string     `json:"original_url"`
	Title       *string    `json:"title,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	MaxClicks   *int64     `json:"max_clicks,omitempty"`
}

// Response is the JSON representation of a link returned by the API.
type Response struct {
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

// toResponse converts a domain.Link to its API JSON shape.
func toResponse(link *domain.Link, baseURL string) Response {
	return Response{
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
