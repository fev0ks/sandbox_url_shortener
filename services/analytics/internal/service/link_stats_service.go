package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

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
	s.log.DebugContext(ctx, "loading analytics stats", slog.String("link_id", id.String()))

	stats, err := s.repo.GetByLinkID(ctx, id, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("analytics stats service get by id: %w", err)
	}
	return stats, nil
}
