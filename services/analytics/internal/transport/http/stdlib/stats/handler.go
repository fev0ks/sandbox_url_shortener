package stats

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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

type Response struct {
	TotalClicks   uint64            `json:"total_clicks"`
	LastVisitedAt *time.Time        `json:"last_visited_at,omitempty"`
	Hourly        []HourlyBucket    `json:"hourly"`
	TopReferrers  []ReferrerSummary `json:"top_referrers"`
}

type HourlyBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	Clicks      uint64    `json:"clicks"`
}

type ReferrerSummary struct {
	Referer string `json:"referer"`
	Clicks  uint64 `json:"clicks"`
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

func toResponse(stats *domain.LinkStats) Response {
	resp := Response{
		TotalClicks:   stats.TotalClicks,
		LastVisitedAt: stats.LastVisitedAt,
		Hourly:        make([]HourlyBucket, 0, len(stats.Hourly)),
		TopReferrers:  make([]ReferrerSummary, 0, len(stats.TopReferrers)),
	}
	for _, bucket := range stats.Hourly {
		resp.Hourly = append(resp.Hourly, HourlyBucket{
			BucketStart: bucket.BucketStart,
			Clicks:      bucket.Clicks,
		})
	}
	for _, ref := range stats.TopReferrers {
		resp.TopReferrers = append(resp.TopReferrers, ReferrerSummary{
			Referer: ref.Referer,
			Clicks:  ref.Clicks,
		})
	}
	return resp
}
