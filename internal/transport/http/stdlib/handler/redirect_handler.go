package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"sandbox_url_shortener/internal/domain"
)

// RedirectHandler handles short code resolution and HTTP redirects.
type RedirectHandler struct {
	svc domain.LinkService
	log *slog.Logger
}

// NewRedirectHandler creates a new redirect handler.
func NewRedirectHandler(svc domain.LinkService, log *slog.Logger) *RedirectHandler {
	return &RedirectHandler{svc: svc, log: log}
}

// Redirect handles GET /{shortCode} — resolves and redirects.
func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("shortCode")

	link, err := h.svc.Resolve(r.Context(), shortCode)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, "Not Found", http.StatusNotFound)
	case errors.Is(err, domain.ErrLinkGone):
		http.Error(w, "Gone", http.StatusGone)
	case err != nil:
		h.log.ErrorContext(r.Context(), "failed to resolve link", "error", err, "short_code", shortCode)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	default:
		http.Redirect(w, r, link.OriginalURL, http.StatusFound)
	}
}
