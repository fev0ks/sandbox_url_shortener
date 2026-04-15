package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"sandbox_url_shortener/services/analytics/internal/domain"
)

type LinkStatsService struct {
	repo domain.LinkStatsRepository
	log  *slog.Logger
}

func NewLinkStatsService(repo domain.LinkStatsRepository, log *slog.Logger) *LinkStatsService {
	return &LinkStatsService{repo: repo, log: log}
}

func (s *LinkStatsService) GetByLinkID(ctx context.Context, id uuid.UUID) (*domain.LinkStats, error) {
	ctx, span := otel.Tracer("analytics.service").Start(ctx, "analytics.stats.get_by_link_id")
	defer span.End()

	span.SetAttributes(attribute.String("link.id", id.String()))
	s.log.DebugContext(ctx, "loading analytics stats", slog.String("link_id", id.String()))

	stats, err := s.repo.GetByLinkID(ctx, id, time.Now().UTC())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "stats_query_failed")
		return nil, fmt.Errorf("analytics stats service get by id: %w", err)
	}
	span.SetStatus(codes.Ok, "")
	return stats, nil
}
