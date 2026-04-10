package clickhouse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/platform/eventing"
)

type stubExecConn struct {
	execFn func(ctx context.Context, query string, args ...any) error
}

func (s *stubExecConn) Exec(ctx context.Context, query string, args ...any) error {
	return s.execFn(ctx, query, args...)
}

func TestLinkVisitSink_HandleLinkVisited_Success(t *testing.T) {
	occurredAt := time.Date(2026, 4, 10, 10, 0, 0, 0, time.FixedZone("TEST", 3*60*60))
	eventID := uuid.New()
	linkID := uuid.New()
	called := false

	sink := NewLinkVisitSink(&stubExecConn{
		execFn: func(_ context.Context, query string, args ...any) error {
			called = true
			if !strings.Contains(query, "INSERT INTO click_events_raw") {
				t.Fatalf("unexpected query: %s", query)
			}
			if len(args) != 8 {
				t.Fatalf("args len = %d, want 8", len(args))
			}

			gotEventID, ok := args[0].(uuid.UUID)
			if !ok || gotEventID != eventID {
				t.Fatalf("event id = %#v, want %s", args[0], eventID)
			}
			if gotRequestID, ok := args[1].(string); !ok || gotRequestID != "req-1" {
				t.Fatalf("request id = %#v, want req-1", args[1])
			}
			gotLinkID, ok := args[2].(uuid.UUID)
			if !ok || gotLinkID != linkID {
				t.Fatalf("link id = %#v, want %s", args[2], linkID)
			}
			if gotShortCode, ok := args[3].(string); !ok || gotShortCode != "abc1234" {
				t.Fatalf("short code = %#v, want abc1234", args[3])
			}
			gotOccurredAt, ok := args[4].(time.Time)
			if !ok || !gotOccurredAt.Equal(occurredAt.UTC()) {
				t.Fatalf("occurred at = %#v, want %s", args[4], occurredAt.UTC())
			}
			return nil
		},
	})

	err := sink.HandleLinkVisited(context.Background(), &eventing.LinkVisited{
		EventID:    eventID.String(),
		RequestID:  "req-1",
		LinkID:     linkID.String(),
		ShortCode:  "abc1234",
		OccurredAt: occurredAt,
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		Referer:    "https://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected exec call")
	}
}

func TestLinkVisitSink_HandleLinkVisited_RejectsInvalidIDs(t *testing.T) {
	sink := NewLinkVisitSink(&stubExecConn{
		execFn: func(_ context.Context, _ string, _ ...any) error {
			t.Fatal("exec must not be called")
			return nil
		},
	})

	if err := sink.HandleLinkVisited(context.Background(), &eventing.LinkVisited{
		EventID: "bad-event-id",
		LinkID:  uuid.New().String(),
	}); err == nil || !strings.Contains(err.Error(), "parse event_id") {
		t.Fatalf("expected parse event_id error, got %v", err)
	}

	if err := sink.HandleLinkVisited(context.Background(), &eventing.LinkVisited{
		EventID: uuid.New().String(),
		LinkID:  "bad-link-id",
	}); err == nil || !strings.Contains(err.Error(), "parse link_id") {
		t.Fatalf("expected parse link_id error, got %v", err)
	}
}

func TestLinkVisitSink_HandleLinkVisited_WrapsExecError(t *testing.T) {
	execErr := errors.New("clickhouse down")
	sink := NewLinkVisitSink(&stubExecConn{
		execFn: func(_ context.Context, _ string, _ ...any) error {
			return execErr
		},
	})

	err := sink.HandleLinkVisited(context.Background(), &eventing.LinkVisited{
		EventID: uuid.New().String(),
		LinkID:  uuid.New().String(),
	})
	if !errors.Is(err, execErr) {
		t.Fatalf("expected wrapped exec error, got %v", err)
	}
	if !strings.Contains(err.Error(), "insert click event") {
		t.Fatalf("unexpected error text: %v", err)
	}
}
