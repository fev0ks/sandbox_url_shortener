// Package redirect serves the public short-code redirect endpoint.
package redirect

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"sandbox_url_shortener/internal/platform/httppolicy"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/requestmeta"
	"sandbox_url_shortener/services/shortener/internal/domain"
)

// Handler holds the dependencies needed to resolve and redirect short codes.
type Handler struct {
	svc Resolver
	log *slog.Logger
}

// Resolver is the exact capability the redirect transport needs.
type Resolver interface {
	ResolveAndTrackVisit(ctx context.Context, shortCode string, visit domain.VisitMetadata) (*domain.Link, error)
}

// NewHandler creates a new redirect HTTP handler.
func NewHandler(svc Resolver, log *slog.Logger) *Handler {
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
	h.log.DebugContext(r.Context(), "redirect requested",
		slog.String("operation", "redirect_resolve"),
		slog.String("short_code", shortCode),
	)

	link, err := h.svc.ResolveAndTrackVisit(r.Context(), shortCode, domain.VisitMetadata{
		RequestID:  requestmeta.RequestID(r.Context()),
		OccurredAt: time.Now().UTC(),
		IP:         httppolicy.ClientIP(r),
		UserAgent:  r.UserAgent(),
		Referer:    r.Referer(),
	})
	switch {
	case errors.Is(err, domain.ErrNotFound):
		metrics.ObserveRedirectResolve("shortener", "not_found")
		h.log.InfoContext(r.Context(), "redirect target not found",
			slog.String("operation", "redirect_resolve"),
			slog.String("error_kind", "not_found"),
			slog.String("short_code", shortCode),
		)
		http.Error(w, "Not Found", http.StatusNotFound)
	case errors.Is(err, domain.ErrLinkGone):
		metrics.ObserveRedirectResolve("shortener", "gone")
		h.log.InfoContext(r.Context(), "redirect target gone",
			slog.String("operation", "redirect_resolve"),
			slog.String("error_kind", "link_gone"),
			slog.String("short_code", shortCode),
		)
		http.Error(w, "Gone", http.StatusGone)
	case err != nil:
		metrics.ObserveRedirectResolve("shortener", "error")
		h.log.ErrorContext(r.Context(), "failed to resolve link",
			slog.String("operation", "redirect_resolve"),
			slog.String("error_kind", "resolve_failed"),
			"error", err,
			"short_code", shortCode,
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	default:
		metrics.ObserveRedirectResolve("shortener", "success")
		h.log.DebugContext(r.Context(), "redirect resolved",
			slog.String("operation", "redirect_resolve"),
			slog.String("short_code", shortCode),
			slog.String("link_id", link.ID.String()),
		)
		http.Redirect(w, r, link.OriginalURL, http.StatusFound)
	}
}
