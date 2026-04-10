// Package service implements business logic for the URL shortener.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/platform/eventing"
	"sandbox_url_shortener/services/shortener/internal/domain"
	"sandbox_url_shortener/services/shortener/internal/shortcode"
)

const maxCollisionRetries = 5
const recentLinksLimit = 20

// cacheTTLCap is the maximum lifetime of a cached redirect entry. The
// effective TTL is min(cacheTTLCap, time_until_expires_at).
const cacheTTLCap = 24 * time.Hour

// LinkService implements the shortener business use cases consumed by transport handlers.
type LinkService struct {
	repo           domain.LinkRepository
	cache          domain.LinkCache // optional; nil disables the cache path
	visitPublisher LinkVisitPublisher
	log            *slog.Logger
}

// NewLinkService creates a new link service. Pass a nil cache to disable
// cache-aside; the service then talks to the repository directly.
func NewLinkService(repo domain.LinkRepository, cache domain.LinkCache, visitPublisher LinkVisitPublisher, log *slog.Logger) *LinkService {
	return &LinkService{repo: repo, cache: cache, visitPublisher: visitPublisher, log: log}
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

// ListRecent returns the most recently created non-deleted links.
func (s *LinkService) ListRecent(ctx context.Context) ([]*domain.Link, error) {
	s.log.DebugContext(ctx, "listing recent links", slog.Int("limit", recentLinksLimit))
	links, err := s.repo.ListRecent(ctx, recentLinksLimit)
	if err != nil {
		return nil, fmt.Errorf("link service list recent: %w", err)
	}
	return links, nil
}

// Disable marks a link as disabled and invalidates any cached redirect entry.
func (s *LinkService) Disable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	s.log.DebugContext(ctx, "disabling link in service", slog.String("link_id", id.String()))
	shortCode, err := s.repo.Disable(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, shortCode, "disable")
	link, err := s.repo.GetByID(ctx, id)
	if err == nil {
		s.log.DebugContext(ctx, "link disabled in service", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	}
	return link, err
}

// Enable re-enables a previously disabled link and invalidates any cached entry
// so the next redirect re-warms the cache from PostgreSQL.
func (s *LinkService) Enable(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	s.log.DebugContext(ctx, "enabling link in service", slog.String("link_id", id.String()))
	shortCode, err := s.repo.Enable(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, shortCode, "enable")
	link, err := s.repo.GetByID(ctx, id)
	if err == nil {
		s.log.DebugContext(ctx, "link enabled in service", slog.String("link_id", link.ID.String()), slog.String("short_code", link.ShortCode))
	}
	return link, err
}

// SoftDelete marks a link as deleted and invalidates any cached entry.
func (s *LinkService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	s.log.DebugContext(ctx, "soft deleting link in service", slog.String("link_id", id.String()))
	shortCode, err := s.repo.SoftDelete(ctx, id)
	if err != nil {
		return err
	}
	s.invalidate(ctx, shortCode, "delete")
	return nil
}

// invalidate is a best-effort cache.Delete. Errors are logged and swallowed
// so cache flakiness can never block a state-change response.
func (s *LinkService) invalidate(ctx context.Context, shortCode, op string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Delete(ctx, shortCode); err != nil {
		s.log.WarnContext(ctx, "cache invalidation failed",
			"short_code", shortCode, "op", op, "error", err)
	}
}

// Resolve resolves a short code for redirect.
//
// Cache-aside flow:
//  1. Try the cache. On error, log warn and treat as miss (fail-open).
//  2. On hit AND MaxClicks == 0 AND not expired, increment the authoritative
//     redirect counter in PostgreSQL via a lightweight touch query, then
//     synthesize a domain.Link from the cache payload.
//  3. Otherwise (miss, expired-cache, or capped link) fall through to the
//     repository, which atomically increments redirect_count, then warm the
//     cache best-effort.
func (s *LinkService) Resolve(ctx context.Context, shortCode string) (*domain.Link, error) {
	s.log.DebugContext(ctx, "resolving short code", slog.String("short_code", shortCode))

	now := time.Now().UTC()
	if link, ok, err := s.resolveFromCache(ctx, shortCode, now); err != nil {
		return nil, err
	} else if ok {
		return link, nil
	}

	link, err := s.repo.ResolveAndIncrement(ctx, shortCode)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.warmCache(ctx, link, now)
	}

	s.log.DebugContext(ctx, "short code resolved",
		slog.String("short_code", shortCode),
		slog.String("link_id", link.ID.String()),
		slog.Int64("redirect_count", link.RedirectCount),
	)
	return link, nil
}

func (s *LinkService) resolveFromCache(ctx context.Context, shortCode string, now time.Time) (*domain.Link, bool, error) {
	if s.cache == nil {
		return nil, false, nil
	}

	cached, err := s.cache.Get(ctx, shortCode)
	if err != nil {
		s.log.WarnContext(ctx, "link cache get failed; falling back to db",
			"error", err, "short_code", shortCode)
		return nil, false, nil
	}
	if cached == nil || cached.MaxClicks != 0 || isExpired(cached.ExpiresAt, now) {
		return nil, false, nil
	}

	linkID, err := uuid.Parse(cached.LinkID)
	if err != nil {
		s.log.WarnContext(ctx, "cached link id invalid; falling back to db",
			"error", err, "short_code", shortCode)
		return nil, false, nil
	}

	if err := s.repo.TouchRedirect(ctx, shortCode); err != nil {
		return nil, false, err
	}

	s.log.DebugContext(ctx, "redirect served from cache after touch", slog.String("short_code", shortCode))

	return &domain.Link{
		ID:          linkID,
		ShortCode:   shortCode,
		OriginalURL: cached.OriginalURL,
		MaxClicks:   cached.MaxClicks,
		ExpiresAt:   cached.ExpiresAt,
	}, true, nil
}

// ResolveAndTrackVisit resolves a short code for redirect and then publishes a
// best-effort link.visited event. Publish failures are logged and swallowed so
// redirect correctness never depends on Kafka availability.
func (s *LinkService) ResolveAndTrackVisit(ctx context.Context, shortCode string, visit domain.VisitMetadata) (*domain.Link, error) {
	link, err := s.Resolve(ctx, shortCode)
	if err != nil {
		return nil, err
	}
	if s.visitPublisher == nil {
		return link, nil
	}

	occurredAt := visit.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	event, err := eventing.NewLinkVisited(eventing.LinkVisitedParams{
		EventID:    uuid.NewString(),
		RequestID:  visit.RequestID,
		LinkID:     link.ID.String(),
		ShortCode:  link.ShortCode,
		OccurredAt: occurredAt,
		IP:         visit.IP,
		UserAgent:  visit.UserAgent,
		Referer:    visit.Referer,
	})
	if err != nil {
		s.log.WarnContext(ctx, "link.visited build failed; skipping publish",
			"error", err, "short_code", shortCode)
		return link, nil
	}

	if err := s.visitPublisher.PublishLinkVisited(ctx, event); err != nil {
		s.log.WarnContext(ctx, "link.visited publish failed",
			"error", err,
			"short_code", shortCode,
			"link_id", link.ID.String(),
			"event_id", event.EventID,
		)
		return link, nil
	}

	s.log.DebugContext(ctx, "link.visited published",
		slog.String("short_code", shortCode),
		slog.String("link_id", link.ID.String()),
		slog.String("event_id", event.EventID),
	)

	return link, nil
}

// warmCache writes the resolved link into the cache best-effort. Errors are
// logged and swallowed — they must never affect the redirect response.
func (s *LinkService) warmCache(ctx context.Context, link *domain.Link, now time.Time) {
	ttl := computeTTL(link.ExpiresAt, now)
	if ttl <= 0 {
		return
	}
	entry := &domain.CachedLink{
		LinkID:      link.ID.String(),
		OriginalURL: link.OriginalURL,
		MaxClicks:   link.MaxClicks,
		ExpiresAt:   link.ExpiresAt,
	}
	if err := s.cache.Set(ctx, link.ShortCode, entry, ttl); err != nil {
		s.log.WarnContext(ctx, "link cache set failed",
			"error", err, "short_code", link.ShortCode)
	}
}

// computeTTL returns min(cacheTTLCap, expiresAt - now) with a floor of zero.
// A nil expiresAt yields the cap.
func computeTTL(expiresAt *time.Time, now time.Time) time.Duration {
	if expiresAt == nil {
		return cacheTTLCap
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}
	if remaining > cacheTTLCap {
		return cacheTTLCap
	}
	return remaining
}

// isExpired reports whether expiresAt has passed. A nil expiresAt never expires.
func isExpired(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && !expiresAt.After(now)
}
