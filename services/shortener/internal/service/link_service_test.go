package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/shortener/internal/domain"
)

// mockLinkRepo is a test double for domain.LinkRepository.
type mockLinkRepo struct {
	createFn              func(ctx context.Context, link *domain.Link) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Link, error)
	disableFn             func(ctx context.Context, id uuid.UUID) error
	enableFn              func(ctx context.Context, id uuid.UUID) error
	softDeleteFn          func(ctx context.Context, id uuid.UUID) error
	resolveAndIncrementFn func(ctx context.Context, shortCode string) (*domain.Link, error)
}

func (m *mockLinkRepo) Create(ctx context.Context, link *domain.Link) error {
	return m.createFn(ctx, link)
}

func (m *mockLinkRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Link, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockLinkRepo) Disable(ctx context.Context, id uuid.UUID) error {
	return m.disableFn(ctx, id)
}

func (m *mockLinkRepo) Enable(ctx context.Context, id uuid.UUID) error {
	return m.enableFn(ctx, id)
}

func (m *mockLinkRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.softDeleteFn(ctx, id)
}

func (m *mockLinkRepo) ResolveAndIncrement(ctx context.Context, shortCode string) (*domain.Link, error) {
	return m.resolveAndIncrementFn(ctx, shortCode)
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
	svc := NewLinkService(repo, testLogger())

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
	svc := NewLinkService(repo, testLogger())

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
	svc := NewLinkService(repo, testLogger())

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
	svc := NewLinkService(repo, testLogger())

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestResolve_Success(t *testing.T) {
	expected := &domain.Link{ShortCode: "abc1234", OriginalURL: "https://example.com"}
	repo := &mockLinkRepo{
		resolveAndIncrementFn: func(_ context.Context, _ string) (*domain.Link, error) {
			return expected, nil
		},
	}
	svc := NewLinkService(repo, testLogger())

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
	svc := NewLinkService(repo, testLogger())

	_, err := svc.Resolve(context.Background(), "abc1234")
	if !errors.Is(err, domain.ErrLinkGone) {
		t.Errorf("expected ErrLinkGone, got: %v", err)
	}
}

func TestSoftDelete_NotFound(t *testing.T) {
	repo := &mockLinkRepo{
		softDeleteFn: func(_ context.Context, _ uuid.UUID) error {
			return domain.ErrNotFound
		},
	}
	svc := NewLinkService(repo, testLogger())

	err := svc.SoftDelete(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}
