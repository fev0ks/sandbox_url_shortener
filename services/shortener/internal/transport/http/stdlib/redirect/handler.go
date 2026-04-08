// Package redirect serves the public short-code redirect endpoint.
package redirect

import (
	"errors"
	"log/slog"
	"net/http"

	"sandbox_url_shortener/services/shortener/internal/domain"
)

// Handler holds the dependencies needed to resolve and redirect short codes.
type Handler struct {
	svc domain.LinkService
	log *slog.Logger
}

// NewHandler creates a new redirect HTTP handler.
func NewHandler(svc domain.LinkService, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register wires the redirect catch-all route onto the given mux.
//
// Note: Go 1.22+ http.ServeMux resolves by pattern specificity, not by
// registration order, so this catch-all does NOT shadow more specific
// routes registered elsewhere.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{shortCode}", h.Redirect)
}

// Redirect handles GET /{shortCode} — resolves the code and 302s to the original URL.
func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("shortCode")
	h.log.DebugContext(r.Context(), "redirect requested", slog.String("short_code", shortCode))

	link, err := h.svc.Resolve(r.Context(), shortCode)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		h.log.InfoContext(r.Context(), "redirect target not found", slog.String("short_code", shortCode))
		http.Error(w, "Not Found", http.StatusNotFound)
	case errors.Is(err, domain.ErrLinkGone):
		h.log.InfoContext(r.Context(), "redirect target gone", slog.String("short_code", shortCode))
		http.Error(w, "Gone", http.StatusGone)
	case err != nil:
		h.log.ErrorContext(r.Context(), "failed to resolve link", "error", err, "short_code", shortCode)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	default:
		h.log.DebugContext(r.Context(), "redirect resolved",
			slog.String("short_code", shortCode),
			slog.String("link_id", link.ID.String()),
		)
		http.Redirect(w, r, link.OriginalURL, http.StatusFound)
	}
}
