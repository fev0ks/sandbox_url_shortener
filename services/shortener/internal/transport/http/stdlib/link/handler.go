// Package link exposes the link CRUD HTTP API.
//
// Each feature owns its own subpackage under transport/http/stdlib/. The
// package contains a Handler struct (its dependencies), a constructor, a
// Register method that wires its routes onto an http.ServeMux, and the
// handler methods themselves, all in one file unless it gets unwieldy.
package link

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/shortener/internal/domain"
	"sandbox_url_shortener/services/shortener/internal/transport/http/stdlib/respond"
)

// Handler holds the dependencies needed to serve the link CRUD API.
type Handler struct {
	svc     domain.LinkService
	log     *slog.Logger
	baseURL string
}

// NewHandler creates a new link HTTP handler.
func NewHandler(svc domain.LinkService, log *slog.Logger, baseURL string) *Handler {
	return &Handler{svc: svc, log: log, baseURL: baseURL}
}

// Register wires the link routes onto the given mux.
//
// All routes for this feature live here so a single grep tells you the
// HTTP surface of the link domain.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/links", h.Create)
	mux.HandleFunc("GET /api/v1/links/{id}", h.GetByID)
	mux.HandleFunc("POST /api/v1/links/{id}/disable", h.Disable)
	mux.HandleFunc("POST /api/v1/links/{id}/enable", h.Enable)
	mux.HandleFunc("DELETE /api/v1/links/{id}", h.Delete)
}

// Create handles POST /api/v1/links.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.DebugContext(r.Context(), "create link request rejected: invalid json")
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_json", "malformed JSON body")
		return
	}

	if strings.TrimSpace(req.OriginalURL) == "" {
		h.log.DebugContext(r.Context(), "create link request rejected: original_url missing")
		respond.Error(r.Context(), w, http.StatusBadRequest, "validation_error", "original_url is required")
		return
	}

	input := domain.CreateLinkInput{
		OriginalURL: req.OriginalURL,
		Title:       req.Title,
		ExpiresAt:   req.ExpiresAt,
	}
	if req.MaxClicks != nil {
		input.MaxClicks = *req.MaxClicks
	}

	h.log.DebugContext(r.Context(), "create link request accepted",
		slog.Bool("has_title", req.Title != nil),
		slog.Bool("has_expires_at", req.ExpiresAt != nil),
		slog.Int64("max_clicks", input.MaxClicks),
	)

	link, err := h.svc.Create(r.Context(), input)
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to create link", "error", err)
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to create link")
		return
	}

	h.log.InfoContext(r.Context(), "link created",
		slog.String("link_id", link.ID.String()),
		slog.String("short_code", link.ShortCode),
		slog.Int64("max_clicks", link.MaxClicks),
		slog.Bool("has_expires_at", link.ExpiresAt != nil),
	)
	respond.JSON(w, http.StatusCreated, toResponse(link, h.baseURL))
}

// GetByID handles GET /api/v1/links/{id}.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.log.DebugContext(r.Context(), "get link request rejected: invalid id", slog.String("id", idStr))
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	h.log.DebugContext(r.Context(), "fetching link by id", slog.String("link_id", id.String()))
	link, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), "link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to get link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to get link")
		return
	}

	h.log.DebugContext(r.Context(), "link fetched", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	respond.JSON(w, http.StatusOK, toResponse(link, h.baseURL))
}

// Disable handles POST /api/v1/links/{id}/disable.
func (h *Handler) Disable(w http.ResponseWriter, r *http.Request) {
	h.changeState(w, r, "disable", h.svc.Disable)
}

// Enable handles POST /api/v1/links/{id}/enable.
func (h *Handler) Enable(w http.ResponseWriter, r *http.Request) {
	h.changeState(w, r, "enable", h.svc.Enable)
}

// changeState applies a state transition like enable/disable and logs the outcome.
func (h *Handler) changeState(
	w http.ResponseWriter,
	r *http.Request,
	action string,
	changeFn func(context.Context, uuid.UUID) (*domain.Link, error),
) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	h.log.DebugContext(r.Context(), action+" link", slog.String("link_id", id.String()))
	link, err := changeFn(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), action+" skipped: link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to "+action+" link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to "+action+" link")
		return
	}

	h.log.InfoContext(r.Context(), "link "+action+"d",
		slog.String("link_id", link.ID.String()),
		slog.String("short_code", link.ShortCode),
	)
	respond.JSON(w, http.StatusOK, toResponse(link, h.baseURL))
}

// Delete handles DELETE /api/v1/links/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	h.log.DebugContext(r.Context(), "deleting link", slog.String("link_id", id.String()))
	err = h.svc.SoftDelete(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.InfoContext(r.Context(), "delete skipped: link not found", slog.String("link_id", id.String()))
			respond.Error(r.Context(), w, http.StatusNotFound, "not_found", "link not found")
			return
		}
		h.log.ErrorContext(r.Context(), "failed to delete link", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to delete link")
		return
	}

	h.log.InfoContext(r.Context(), "link deleted", slog.String("link_id", id.String()))
	w.WriteHeader(http.StatusNoContent)
}

// parseID extracts and validates the {id} path parameter.
func (h *Handler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.log.DebugContext(r.Context(), "request rejected: invalid uuid", slog.String("id", idStr))
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return uuid.Nil, err
	}
	return id, nil
}
