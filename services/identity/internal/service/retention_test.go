package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestRetentionServiceCleanupUsesConfiguredCutoffs(t *testing.T) {
	repo := newFakeRepo()
	repo.oauthCleanupDeleted = []int64{2, 1}
	repo.sessionCleanupDeleted = []int64{2, 2, 1}
	svc := NewRetentionService(repo, slog.Default(), 24*time.Hour, 30*24*time.Hour, 2)
	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

	result, err := svc.Cleanup(context.Background(), now)
	if err != nil {
		t.Fatalf("Cleanup error: %v", err)
	}
	if result.OAuthStatesDeleted != 3 {
		t.Fatalf("OAuthStatesDeleted = %d", result.OAuthStatesDeleted)
	}
	if result.SessionsDeleted != 5 {
		t.Fatalf("SessionsDeleted = %d", result.SessionsDeleted)
	}
	if len(repo.oauthCleanupLimits) != 2 {
		t.Fatalf("oauth cleanup calls = %d", len(repo.oauthCleanupLimits))
	}
	if len(repo.sessionCleanupLimits) != 3 {
		t.Fatalf("session cleanup calls = %d", len(repo.sessionCleanupLimits))
	}
	if want := now.Add(-24 * time.Hour); !repo.oauthCleanupCutoff.Equal(want) {
		t.Fatalf("oauth cutoff = %s, want %s", repo.oauthCleanupCutoff, want)
	}
	if want := now.Add(-30 * 24 * time.Hour); !repo.sessionCleanupCutoff.Equal(want) {
		t.Fatalf("session cutoff = %s, want %s", repo.sessionCleanupCutoff, want)
	}
}

func TestRetentionServiceCleanupWrapsRepositoryErrors(t *testing.T) {
	repo := newFakeRepo()
	repo.oauthCleanupErr = errors.New("oauth cleanup failed")
	svc := NewRetentionService(repo, slog.Default(), time.Hour, 24*time.Hour, 100)

	if _, err := svc.Cleanup(context.Background(), time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected oauth cleanup error")
	}

	repo.oauthCleanupErr = nil
	repo.sessionCleanupErr = errors.New("session cleanup failed")
	if _, err := svc.Cleanup(context.Background(), time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected session cleanup error")
	}
}
