package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/analytics/internal/domain"
)

type stubLinkStatsRepo struct {
	getByLinkIDFn func(ctx context.Context, linkID uuid.UUID, now time.Time) (*domain.LinkStats, error)
}

func (s *stubLinkStatsRepo) GetByLinkID(ctx context.Context, linkID uuid.UUID, now time.Time) (*domain.LinkStats, error) {
	return s.getByLinkIDFn(ctx, linkID, now)
}

func quietStatsLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestLinkStatsService_GetByLinkID_Success(t *testing.T) {
	linkID := uuid.New()
	expected := &domain.LinkStats{TotalClicks: 7}
	called := false

	svc := NewLinkStatsService(&stubLinkStatsRepo{
		getByLinkIDFn: func(_ context.Context, gotID uuid.UUID, now time.Time) (*domain.LinkStats, error) {
			called = true
			if gotID != linkID {
				t.Fatalf("link id = %s, want %s", gotID, linkID)
			}
			if now.IsZero() {
				t.Fatal("expected non-zero time")
			}
			return expected, nil
		},
	}, quietStatsLogger())

	got, err := svc.GetByLinkID(context.Background(), linkID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected repository call")
	}
	if got != expected {
		t.Fatalf("stats pointer mismatch")
	}
}

func TestLinkStatsService_GetByLinkID_WrapsRepoError(t *testing.T) {
	linkID := uuid.New()
	repoErr := errors.New("clickhouse unavailable")
	svc := NewLinkStatsService(&stubLinkStatsRepo{
		getByLinkIDFn: func(_ context.Context, _ uuid.UUID, _ time.Time) (*domain.LinkStats, error) {
			return nil, repoErr
		},
	}, quietStatsLogger())

	_, err := svc.GetByLinkID(context.Background(), linkID)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, repoErr) {
		t.Fatalf("expected wrapped repo error, got %v", err)
	}
	if !strings.Contains(err.Error(), "analytics stats service get by id") {
		t.Fatalf("unexpected error text: %v", err)
	}
}
