package stats

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/analytics/internal/domain"
	"sandbox_url_shortener/services/analytics/internal/transport/http/stdlib/respond"
)

type Handler struct {
	svc Service
	log *slog.Logger
}

type Service interface {
	GetByLinkID(ctx context.Context, id uuid.UUID) (*domain.LinkStats, error)
}

func NewHandler(svc Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/links/{id}/stats", h.GetByLinkID)
}

func (h *Handler) GetByLinkID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.log.DebugContext(r.Context(), "get analytics stats rejected: invalid id", slog.String("id", idStr))
		respond.Error(r.Context(), w, http.StatusBadRequest, "invalid_id", "id must be a valid UUID")
		return
	}

	stats, err := h.svc.GetByLinkID(r.Context(), id)
	if err != nil {
		h.log.ErrorContext(r.Context(), "failed to load analytics stats", "error", err, slog.String("link_id", id.String()))
		respond.Error(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to load link stats")
		return
	}

	respond.JSON(w, http.StatusOK, toResponse(stats))
}
