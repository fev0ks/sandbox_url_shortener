// Package service implements business logic for the URL shortener.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/domain"
	"sandbox_url_shortener/internal/shortcode"
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

		err = s.repo.Create(ctx, link)
		if err == nil {
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
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return link, nil
}

// Disable marks a link as disabled.
func (s *LinkService) Disable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	if err := s.repo.Disable(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// Enable re-enables a previously disabled link.
func (s *LinkService) Enable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	if err := s.repo.Enable(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// SoftDelete marks a link as deleted.
func (s *LinkService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, id)
}

// Resolve resolves a short code for redirect.
func (s *LinkService) Resolve(ctx context.Context, shortCode string) (*domain.Link, error) {
	return s.repo.ResolveAndIncrement(ctx, shortCode)
}
