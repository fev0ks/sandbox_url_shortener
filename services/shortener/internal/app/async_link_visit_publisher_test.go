package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"sandbox_url_shortener/internal/platform/eventing"
)

type stubSyncLinkVisitPublisher struct {
	err error
}

func (s *stubSyncLinkVisitPublisher) PublishLinkVisited(context.Context, *eventing.LinkVisited) error {
	return s.err
}

func (s *stubSyncLinkVisitPublisher) Close() error {
	return nil
}

func TestAsyncLinkVisitPublisher_LogsStructuredFailureFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	publisher := newAsyncLinkVisitPublisher(context.Background(), logger, &stubSyncLinkVisitPublisher{
		err: errors.New("kafka unavailable"),
	})

	event, err := eventing.NewLinkVisited(eventing.LinkVisitedParams{
		EventID:    "11111111-1111-1111-1111-111111111111",
		RequestID:  "req-e07",
		LinkID:     "22222222-2222-2222-2222-222222222222",
		ShortCode:  "abc1234",
		OccurredAt: time.Now().UTC(),
		IP:         "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("new link visited event: %v", err)
	}

	if err := publisher.PublishLinkVisited(context.Background(), event); err != nil {
		t.Fatalf("enqueue publish: %v", err)
	}
	if err := publisher.Close(); err != nil {
		t.Fatalf("close publisher: %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("unmarshal log entry: %v", err)
	}

	if got := entry["msg"]; got != "async link.visited publish failed" {
		t.Fatalf("msg = %v, want async link.visited publish failed", got)
	}
	if got := entry["operation"]; got != "publish_link_visited" {
		t.Fatalf("operation = %v, want publish_link_visited", got)
	}
	if got := entry["error_kind"]; got != "publish_failed" {
		t.Fatalf("error_kind = %v, want publish_failed", got)
	}
	if got := entry["event_id"]; got != event.EventID {
		t.Fatalf("event_id = %v, want %s", got, event.EventID)
	}
	if got := entry["link_id"]; got != event.LinkID {
		t.Fatalf("link_id = %v, want %s", got, event.LinkID)
	}
	if got := entry["short_code"]; got != event.ShortCode {
		t.Fatalf("short_code = %v, want %s", got, event.ShortCode)
	}
}
