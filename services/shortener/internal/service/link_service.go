// Package service implements business logic for the URL shortener.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/shortener/internal/domain"
	"sandbox_url_shortener/services/shortener/internal/shortcode"
)

const maxCollisionRetries = 5

// LinkService implements domain.LinkService.
type LinkService struct {
	repo domain.LinkRepository
	log  *slog.Logger
}

// NewLinkService creates a new link service.
func NewLinkService(repo domain.LinkRepository, log *slog.Logger) *LinkService {
	return &LinkService{repo: repo, log: log}
}

// Create generates a UUIDv7 and short code, then persists a new link.
// Retries short code generation up to 5 times on collision.
func (s *LinkService) Create(ctx context.Context, input domain.CreateLinkInput) (*domain.Link, error) {
	s.log.DebugContext(ctx, "creating link",
		slog.Bool("has_title", input.Title != nil),
		slog.Bool("has_expires_at", input.ExpiresAt != nil),
		slog.Int64("max_clicks", input.MaxClicks),
	)

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("link service create: generate uuid: %w", err)
	}

	now := time.Now().UTC()
	link := &domain.Link{
		ID:          id,
		OriginalURL: input.OriginalURL,
		Title:       input.Title,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   input.ExpiresAt,
		MaxClicks:   input.MaxClicks,
	}

	for attempt := 1; attempt <= maxCollisionRetries; attempt++ {
		code, err := shortcode.Generate()
		if err != nil {
			return nil, fmt.Errorf("link service create: generate short code: %w", err)
		}
		link.ShortCode = code
		s.log.DebugContext(ctx, "attempting link insert",
			slog.Int("attempt", attempt),
			slog.String("short_code", code),
		)

		err = s.repo.Create(ctx, link)
		if err == nil {
			s.log.DebugContext(ctx, "link persisted",
				slog.String("link_id", link.ID.String()),
				slog.String("short_code", link.ShortCode),
			)
			return link, nil
		}
		if !errors.Is(err, domain.ErrShortCodeCollision) {
			return nil, fmt.Errorf("link service create: %w", err)
		}

		s.log.WarnContext(ctx, "short code collision, retrying",
			"attempt", attempt,
			"short_code", code,
		)
	}

	return nil, fmt.Errorf("link service create: exhausted %d collision retries", maxCollisionRetries)
}

// GetByID returns an active link by its internal ID.
func (s *LinkService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	s.log.DebugContext(ctx, "loading link by id", slog.String("link_id", id.String()))
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.log.DebugContext(ctx, "link loaded", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	return link, nil
}

// Disable marks a link as disabled.
func (s *LinkService) Disable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	s.log.DebugContext(ctx, "disabling link in service", slog.String("link_id", id.String()))
	if err := s.repo.Disable(ctx, id); err != nil {
		return nil, err
	}
	link, err := s.repo.GetByID(ctx, id)
	if err == nil {
		s.log.DebugContext(ctx, "link disabled in service", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	}
	return link, err
}

// Enable re-enables a previously disabled link.
func (s *LinkService) Enable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	s.log.DebugContext(ctx, "enabling link in service", slog.String("link_id", id.String()))
	if err := s.repo.Enable(ctx, id); err != nil {
		return nil, err
	}
	link, err := s.repo.GetByID(ctx, id)
	if err == nil {
		s.log.DebugContext(ctx, "link enabled in service", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	}
	return link, err
}

// SoftDelete marks a link as deleted.
func (s *LinkService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	s.log.DebugContext(ctx, "soft deleting link in service", slog.String("link_id", id.String()))
	return s.repo.SoftDelete(ctx, id)
}

// Resolve resolves a short code for redirect.
func (s *LinkService) Resolve(ctx context.Context, shortCode string) (*domain.Link, error) {
	s.log.DebugContext(ctx, "resolving short code", slog.String("short_code", shortCode))
	link, err := s.repo.ResolveAndIncrement(ctx, shortCode)
	if err != nil {
		return nil, err
	}
	s.log.DebugContext(ctx, "short code resolved",
		slog.String("short_code", shortCode),
		slog.String("link_id", link.ID.String()),
		slog.Int64("redirect_count", link.RedirectCount),
	)
	return link, nil
}
