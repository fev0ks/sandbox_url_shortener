package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/domain"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/transport/http/stdlib/respond"
)

// LinkHandler handles HTTP requests for link CRUD operations.
type LinkHandler struct {
	svc     domain.LinkService
	log     *slog.Logger
	baseURL string
}

// NewLinkHandler creates a new link handler.
func NewLinkHandler(svc domain.LinkService, log *slog.Logger, baseURL string) *LinkHandler {
	return &LinkHandler{svc: svc, log: log, baseURL: baseURL}
}

// Create handles POST /api/v1/links.
func (h *LinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_json", "malformed JSON body", middleware.GetRequestID(r.Context()))
		return
	}

	if strings.TrimSpace(req.OriginalURL) == "" {
		respond.Error(w, http.StatusBadRequest, "validation_error", "original_url is required", middleware.GetRequestID(r.Context()))
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

	link, err := h.svc.Create(r.Context(), input)
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to create link", "error", err)
		respond.Error(w, http.StatusInternalServerError, "internal_error", "failed to create link", middleware.GetRequestID(r.Context()))
		return
	}

	respond.JSON(w, http.StatusCreated, toLinkResponse(link, h.baseURL))
}

// GetByID handles GET /api/v1/links/{id}.
func (h *LinkHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID", middleware.GetRequestID(r.Context()))
		return
	}

	link, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "not_found", "link not found", middleware.GetRequestID(r.Context()))
			return
		}
		h.log.ErrorContext(r.Context(), "failed to get link", "error", err)
		respond.Error(w, http.StatusInternalServerError, "internal_error", "failed to get link", middleware.GetRequestID(r.Context()))
		return
	}

	respond.JSON(w, http.StatusOK, toLinkResponse(link, h.baseURL))
}

// Disable handles POST /api/v1/links/{id}/disable.
func (h *LinkHandler) Disable(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	link, err := h.svc.Disable(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "not_found", "link not found", middleware.GetRequestID(r.Context()))
			return
		}
		h.log.ErrorContext(r.Context(), "failed to disable link", "error", err)
		respond.Error(w, http.StatusInternalServerError, "internal_error", "failed to disable link", middleware.GetRequestID(r.Context()))
		return
	}

	respond.JSON(w, http.StatusOK, toLinkResponse(link, h.baseURL))
}

// Enable handles POST /api/v1/links/{id}/enable.
func (h *LinkHandler) Enable(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	link, err := h.svc.Enable(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "not_found", "link not found", middleware.GetRequestID(r.Context()))
			return
		}
		h.log.ErrorContext(r.Context(), "failed to enable link", "error", err)
		respond.Error(w, http.StatusInternalServerError, "internal_error", "failed to enable link", middleware.GetRequestID(r.Context()))
		return
	}

	respond.JSON(w, http.StatusOK, toLinkResponse(link, h.baseURL))
}

// Delete handles DELETE /api/v1/links/{id}.
func (h *LinkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := h.parseID(w, r)
	if err != nil {
		return
	}

	err = h.svc.SoftDelete(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respond.Error(w, http.StatusNotFound, "not_found", "link not found", middleware.GetRequestID(r.Context()))
			return
		}
		h.log.ErrorContext(r.Context(), "failed to delete link", "error", err)
		respond.Error(w, http.StatusInternalServerError, "internal_error", "failed to delete link", middleware.GetRequestID(r.Context()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseID extracts and validates the {id} path parameter.
func (h *LinkHandler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID", middleware.GetRequestID(r.Context()))
		return uuid.Nil, err
	}
	return id, nil
}
