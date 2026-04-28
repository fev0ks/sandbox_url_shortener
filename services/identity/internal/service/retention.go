package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"sandbox_url_shortener/services/identity/internal/domain"
)

// RetentionService owns explicit identity cleanup workflows.
type RetentionService struct {
	repo                domain.Repository
	log                 *slog.Logger
	oauthStateRetention time.Duration
	sessionRetention    time.Duration
	deleteBatchSize     int
}

// RetentionResult reports the number of rows removed by an identity cleanup run.
type RetentionResult struct {
	OAuthStatesDeleted int64
	SessionsDeleted    int64
	OAuthStateCutoff   time.Time
	SessionCutoff      time.Time
}

// NewRetentionService creates a retention service with explicit retention windows.
func NewRetentionService(repo domain.Repository, log *slog.Logger, oauthStateRetention, sessionRetention time.Duration, deleteBatchSize int) *RetentionService {
	return &RetentionService{
		repo:                repo,
		log:                 log,
		oauthStateRetention: oauthStateRetention,
		sessionRetention:    sessionRetention,
		deleteBatchSize:     deleteBatchSize,
	}
}

// Cleanup deletes expired or consumed OAuth states and revoked or expired sessions.
func (s *RetentionService) Cleanup(ctx context.Context, now time.Time) (*RetentionResult, error) {
	now = normalizedNow(now)
	oauthStateCutoff := now.Add(-s.oauthStateRetention)
	sessionCutoff := now.Add(-s.sessionRetention)

	oauthDeleted, err := s.deleteOAuthStates(ctx, oauthStateCutoff)
	if err != nil {
		return nil, fmt.Errorf("identity retention oauth states: %w", err)
	}
	sessionDeleted, err := s.deleteSessions(ctx, sessionCutoff)
	if err != nil {
		return nil, fmt.Errorf("identity retention sessions: %w", err)
	}

	result := &RetentionResult{
		OAuthStatesDeleted: oauthDeleted,
		SessionsDeleted:    sessionDeleted,
		OAuthStateCutoff:   oauthStateCutoff,
		SessionCutoff:      sessionCutoff,
	}
	s.log.InfoContext(ctx, "identity retention cleanup completed",
		slog.String("operation", OperationIdentityRetentionCleanup),
		slog.Int64("oauth_states_deleted", result.OAuthStatesDeleted),
		slog.Int64("sessions_deleted", result.SessionsDeleted),
		slog.Time("oauth_state_cutoff", result.OAuthStateCutoff),
		slog.Time("session_cutoff", result.SessionCutoff),
		slog.Int("delete_batch_size", s.deleteBatchSize),
	)
	return result, nil
}

func (s *RetentionService) deleteOAuthStates(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		deleted, err := s.repo.DeleteOAuthStatesBefore(ctx, cutoff, s.deleteBatchSize)
		if err != nil {
			return 0, err
		}
		total += deleted
		if deleted < int64(s.deleteBatchSize) {
			return total, nil
		}
	}
}

func (s *RetentionService) deleteSessions(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		deleted, err := s.repo.DeleteSessionsBefore(ctx, cutoff, s.deleteBatchSize)
		if err != nil {
			return 0, err
		}
		total += deleted
		if deleted < int64(s.deleteBatchSize) {
			return total, nil
		}
	}
}
