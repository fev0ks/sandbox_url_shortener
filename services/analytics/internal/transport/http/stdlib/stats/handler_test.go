package stats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/services/analytics/internal/domain"
)

type stubStatsService struct {
	getByLinkIDFn func(ctx context.Context, id uuid.UUID) (*domain.LinkStats, error)
}

func (s *stubStatsService) GetByLinkID(ctx context.Context, id uuid.UUID) (*domain.LinkStats, error) {
	return s.getByLinkIDFn(ctx, id)
}

func quietHandlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestHandler_GetByLinkID_InvalidID(t *testing.T) {
	h := NewHandler(&stubStatsService{
		getByLinkIDFn: func(_ context.Context, _ uuid.UUID) (*domain.LinkStats, error) {
			t.Fatal("service must not be called for invalid id")
			return nil, nil
		},
	}, quietHandlerLogger())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/not-a-uuid/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"]["code"] != "invalid_id" {
		t.Fatalf("error code = %q, want invalid_id", body["error"]["code"])
	}
}

func TestHandler_GetByLinkID_ServiceError(t *testing.T) {
	linkID := uuid.New()
	h := NewHandler(&stubStatsService{
		getByLinkIDFn: func(_ context.Context, got uuid.UUID) (*domain.LinkStats, error) {
			if got != linkID {
				t.Fatalf("link id = %s, want %s", got, linkID)
			}
			return nil, errors.New("boom")
		},
	}, quietHandlerLogger())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/"+linkID.String()+"/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandler_GetByLinkID_Success(t *testing.T) {
	linkID := uuid.New()
	lastVisited := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	h := NewHandler(&stubStatsService{
		getByLinkIDFn: func(_ context.Context, got uuid.UUID) (*domain.LinkStats, error) {
			if got != linkID {
				t.Fatalf("link id = %s, want %s", got, linkID)
			}
			return &domain.LinkStats{
				TotalClicks:   3,
				LastVisitedAt: &lastVisited,
				Hourly: []domain.LinkStatsBucket{
					{BucketStart: lastVisited.Add(-time.Hour), Clicks: 2},
				},
				TopReferrers: []domain.LinkReferrerStat{
					{Referer: "https://example.com", Clicks: 2},
				},
			}, nil
		},
	}, quietHandlerLogger())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/"+linkID.String()+"/stats", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.TotalClicks != 3 {
		t.Fatalf("total clicks = %d, want 3", body.TotalClicks)
	}
	if len(body.Hourly) != 1 || body.Hourly[0].Clicks != 2 {
		t.Fatalf("unexpected hourly payload: %+v", body.Hourly)
	}
	if len(body.TopReferrers) != 1 || body.TopReferrers[0].Referer != "https://example.com" {
		t.Fatalf("unexpected referrers payload: %+v", body.TopReferrers)
	}
}
