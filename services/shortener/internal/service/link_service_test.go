package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/shortener/internal/domain"
)

// mockLinkCache is a test double for domain.LinkCache.
type mockLinkCache struct {
	getFn    func(ctx context.Context, shortCode string) (*domain.CachedLink, error)
	setFn    func(ctx context.Context, shortCode string, link *domain.CachedLink, ttl time.Duration) error
	deleteFn func(ctx context.Context, shortCode string) error

	setCalls    int
	deleteCalls int
}

func (m *mockLinkCache) Get(ctx context.Context, shortCode string) (*domain.CachedLink, error) {
	if m.getFn == nil {
		return nil, nil
	}
	return m.getFn(ctx, shortCode)
}

func (m *mockLinkCache) Set(ctx context.Context, shortCode string, link *domain.CachedLink, ttl time.Duration) error {
	m.setCalls++
	if m.setFn == nil {
		return nil
	}
	return m.setFn(ctx, shortCode, link, ttl)
}

func (m *mockLinkCache) Delete(ctx context.Context, shortCode string) error {
	m.deleteCalls++
	if m.deleteFn == nil {
		return nil
	}
	return m.deleteFn(ctx, shortCode)
}

// mockLinkRepo is a test double for domain.LinkRepository.
type mockLinkRepo struct {
	createFn              func(ctx context.Context, link *domain.Link) error
	listRecentFn          func(ctx context.Context, limit int) ([]*domain.Link, error)
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Link, error)
	disableFn             func(ctx context.Context, id uuid.UUID) (string, error)
	enableFn              func(ctx context.Context, id uuid.UUID) (string, error)
	softDeleteFn          func(ctx context.Context, id uuid.UUID) (string, error)
	resolveAndIncrementFn func(ctx context.Context, shortCode string) (*domain.Link, error)
	touchRedirectFn       func(ctx context.Context, shortCode string) error
}

func (m *mockLinkRepo) Create(ctx context.Context, link *domain.Link) error {
	return m.createFn(ctx, link)
}

func (m *mockLinkRepo) ListRecent(ctx context.Context, limit int) ([]*domain.Link, error) {
	return m.listRecentFn(ctx, limit)
}

func (m *mockLinkRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockLinkRepo) Disable(ctx context.Context, id uuid.UUID) (string, error) {
	return m.disableFn(ctx, id)
}

func (m *mockLinkRepo) Enable(ctx context.Context, id uuid.UUID) (string, error) {
	return m.enableFn(ctx, id)
}

func (m *mockLinkRepo) SoftDelete(ctx context.Context, id uuid.UUID) (string, error) {
	return m.softDeleteFn(ctx, id)
}

func (m *mockLinkRepo) ResolveAndIncrement(ctx context.Context, shortCode string) (*domain.Link, error) {
	return m.resolveAndIncrementFn(ctx, shortCode)
}

func (m *mockLinkRepo) TouchRedirect(ctx context.Context, shortCode string) error {
	if m.touchRedirectFn == nil {
		return nil
	}
	return m.touchRedirectFn(ctx, shortCode)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestCreate_Success(t *testing.T) {
	repo := &mockLinkRepo{
		createFn: func(_ context.Context, _ *domain.Link) error {
			return nil
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	link, err := svc.Create(context.Background(), domain.CreateLinkInput{
		OriginalURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if link.ShortCode == "" {
		t.Error("expected short code to be generated")
	}
	if link.OriginalURL != "https://example.com" {
		t.Errorf("expected original_url %q, got %q", "https://example.com", link.OriginalURL)
	}
	if link.ID == uuid.Nil {
		t.Error("expected non-nil UUID")
	}
}

func TestCreate_CollisionRetry(t *testing.T) {
	callCount := 0
	repo := &mockLinkRepo{
		createFn: func(_ context.Context, _ *domain.Link) error {
			callCount++
			if callCount <= 3 {
				return domain.ErrShortCodeCollision
			}
			return nil
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	link, err := svc.Create(context.Background(), domain.CreateLinkInput{
		OriginalURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link")
	}
	if callCount != 4 {
		t.Errorf("expected 4 create attempts, got %d", callCount)
	}
}

func TestCreate_CollisionExhausted(t *testing.T) {
	repo := &mockLinkRepo{
		createFn: func(_ context.Context, _ *domain.Link) error {
			return domain.ErrShortCodeCollision
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	_, err := svc.Create(context.Background(), domain.CreateLinkInput{
		OriginalURL: "https://example.com",
	})
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &mockLinkRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Link, error) {
			return nil, domain.ErrNotFound
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestListRecent_Success(t *testing.T) {
	expected := []*domain.Link{
		{ShortCode: "newest"},
		{ShortCode: "older"},
	}
	repo := &mockLinkRepo{
		listRecentFn: func(_ context.Context, limit int) ([]*domain.Link, error) {
			if limit != recentLinksLimit {
				t.Fatalf("limit = %d, want %d", limit, recentLinksLimit)
			}
			return expected, nil
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	links, err := svc.ListRecent(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2", len(links))
	}
	if links[0].ShortCode != "newest" {
		t.Fatalf("first short code = %q, want newest", links[0].ShortCode)
	}
}

func TestResolve_Success(t *testing.T) {
	expected := &domain.Link{ShortCode: "abc1234", OriginalURL: "https://example.com"}
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			return expected, nil
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	link, err := svc.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if link.OriginalURL != "https://example.com" {
		t.Errorf("expected original URL %q, got %q", "https://example.com", link.OriginalURL)
	}
}

func TestResolve_Gone(t *testing.T) {
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			return nil, domain.ErrLinkGone
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "abc1234")
	if !errors.Is(err, domain.ErrLinkGone) {
		t.Errorf("expected ErrLinkGone, got: %v", err)
	}
}

func TestSoftDelete_NotFound(t *testing.T) {
	repo := &mockLinkRepo{
		softDeleteFn: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "", domain.ErrNotFound
		},
	}
	svc := NewLinkService(repo, nil, nil, testLogger())

	err := svc.SoftDelete(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestDisable_InvalidatesCache(t *testing.T) {
	repo := &mockLinkRepo{
		disableFn: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "abc1234", nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Link, error) {
			return &domain.Link{ShortCode: "abc1234"}, nil
		},
	}
	cache := &mockLinkCache{}
	svc := NewLinkService(repo, cache, nil, testLogger())

	if _, err := svc.Disable(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cache.deleteCalls != 1 {
		t.Errorf("expected 1 cache.Delete, got %d", cache.deleteCalls)
	}
}

func TestEnable_InvalidatesCache(t *testing.T) {
	repo := &mockLinkRepo{
		enableFn: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "abc1234", nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Link, error) {
			return &domain.Link{ShortCode: "abc1234"}, nil
		},
	}
	cache := &mockLinkCache{}
	svc := NewLinkService(repo, cache, nil, testLogger())

	if _, err := svc.Enable(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cache.deleteCalls != 1 {
		t.Errorf("expected 1 cache.Delete, got %d", cache.deleteCalls)
	}
}

func TestSoftDelete_InvalidatesCache(t *testing.T) {
	repo := &mockLinkRepo{
		softDeleteFn: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "abc1234", nil
		},
	}
	cache := &mockLinkCache{}
	svc := NewLinkService(repo, cache, nil, testLogger())

	if err := svc.SoftDelete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cache.deleteCalls != 1 {
		t.Errorf("expected 1 cache.Delete, got %d", cache.deleteCalls)
	}
}

func TestStateChange_CacheErrorTolerated(t *testing.T) {
	repo := &mockLinkRepo{
		softDeleteFn: func(_ context.Context, _ uuid.UUID) (string, error) {
			return "abc1234", nil
		},
	}
	cache := &mockLinkCache{
		deleteFn: func(_ context.Context, _ string) error {
			return errors.New("redis down")
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	if err := svc.SoftDelete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("expected delete to succeed despite cache error, got: %v", err)
	}
}

func TestResolve_CacheHit_NoDB(t *testing.T) {
	touchCalls := 0
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			t.Fatal("ResolveAndIncrement must not be called on cache hit for unlimited links")
			return nil, nil
		},
		touchRedirectFn: func(_ context.Context, shortCode string) error {
			touchCalls++
			if shortCode != "abc1234" {
				t.Fatalf("unexpected short code %q", shortCode)
			}
			return nil
		},
	}
	cache := &mockLinkCache{
		getFn: func(_ context.Context, _ string) (*domain.CachedLink, error) {
			return &domain.CachedLink{
				LinkID:      "019d6f15-7ff2-7309-b7db-a8d24b35c2fe",
				OriginalURL: "https://example.com",
				MaxClicks:   0,
			}, nil
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	link, err := svc.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.OriginalURL != "https://example.com" {
		t.Errorf("got URL %q", link.OriginalURL)
	}
	if touchCalls != 1 {
		t.Errorf("expected exactly 1 TouchRedirect, got %d", touchCalls)
	}
	if cache.setCalls != 0 {
		t.Errorf("did not expect cache.Set on cache hit")
	}
}

func TestResolve_CacheHit_TouchErrorReturned(t *testing.T) {
	repo := &mockLinkRepo{
		touchRedirectFn: func(_ context.Context, _ string) error {
			return domain.ErrLinkGone
		},
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			t.Fatal("ResolveAndIncrement must not run when touch fails on cache hit")
			return nil, nil
		},
	}
	cache := &mockLinkCache{
		getFn: func(_ context.Context, _ string) (*domain.CachedLink, error) {
			return &domain.CachedLink{
				LinkID:      "019d6f15-7ff2-7309-b7db-a8d24b35c2fe",
				OriginalURL: "https://example.com",
				MaxClicks:   0,
			}, nil
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	_, err := svc.Resolve(context.Background(), "abc1234")
	if !errors.Is(err, domain.ErrLinkGone) {
		t.Fatalf("expected ErrLinkGone, got %v", err)
	}
}

func TestResolve_CacheMiss_DBThenWarm(t *testing.T) {
	expected := &domain.Link{ShortCode: "abc1234", OriginalURL: "https://example.com"}
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			return expected, nil
		},
	}
	cache := &mockLinkCache{
		getFn: func(_ context.Context, _ string) (*domain.CachedLink, error) {
			return nil, nil // miss
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	link, err := svc.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != expected {
		t.Errorf("expected db link returned")
	}
	if cache.setCalls != 1 {
		t.Errorf("expected exactly 1 cache.Set, got %d", cache.setCalls)
	}
}

func TestResolve_CacheError_FallsBackToDB(t *testing.T) {
	expected := &domain.Link{ShortCode: "abc1234", OriginalURL: "https://example.com"}
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			return expected, nil
		},
	}
	cache := &mockLinkCache{
		getFn: func(_ context.Context, _ string) (*domain.CachedLink, error) {
			return nil, errors.New("redis down")
		},
		setFn: func(_ context.Context, _ string, _ *domain.CachedLink, _ time.Duration) error {
			return errors.New("still down")
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	link, err := svc.Resolve(context.Background(), "abc1234")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if link != expected {
		t.Errorf("expected db link")
	}
}

func TestResolve_CacheHit_CapWithMaxClicks_GoesToDB(t *testing.T) {
	dbHit := false
	expected := &domain.Link{ShortCode: "abc1234", OriginalURL: "https://example.com", MaxClicks: 5}
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			dbHit = true
			return expected, nil
		},
	}
	cache := &mockLinkCache{
		getFn: func(_ context.Context, _ string) (*domain.CachedLink, error) {
			return &domain.CachedLink{OriginalURL: "https://example.com", MaxClicks: 5}, nil
		},
	}
	svc := NewLinkService(repo, cache, nil, testLogger())

	if _, err := svc.Resolve(context.Background(), "abc1234"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !dbHit {
		t.Error("expected db path for capped link even on cache hit")
	}
}

func TestComputeTTL(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		exp  *time.Time
		want time.Duration
	}{
		{"nil expires", nil, cacheTTLCap},
		{"far future", ptrTime(now.Add(72 * time.Hour)), cacheTTLCap},
		{"near future", ptrTime(now.Add(30 * time.Minute)), 30 * time.Minute},
		{"already expired", ptrTime(now.Add(-time.Second)), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeTTL(tc.exp, now); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
