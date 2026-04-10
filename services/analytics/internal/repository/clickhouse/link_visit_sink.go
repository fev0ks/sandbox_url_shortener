package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"sandbox_url_shortener/internal/platform/eventing"
)

type execConn interface {
	Exec(ctx context.Context, query string, args ...any) error
}

// LinkVisitSink persists raw link.visited events into ClickHouse.
type LinkVisitSink struct {
	conn execConn
}

func NewLinkVisitSink(conn execConn) *LinkVisitSink {
	return &LinkVisitSink{conn: conn}
}

func (s *LinkVisitSink) HandleLinkVisited(ctx context.Context, event *eventing.LinkVisited) error {
	if event == nil {
		return fmt.Errorf("link.visited event is nil")
	}

	eventID, err := uuid.Parse(strings.TrimSpace(event.EventID))
	if err != nil {
		return fmt.Errorf("parse event_id: %w", err)
	}
	linkID, err := uuid.Parse(strings.TrimSpace(event.LinkID))
	if err != nil {
		return fmt.Errorf("parse link_id: %w", err)
	}

	const q = `
		INSERT INTO click_events_raw (
			event_id,
			request_id,
			link_id,
			short_code,
			occurred_at,
			ip,
			user_agent,
			referer
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	if err := s.conn.Exec(
		ctx,
		q,
		eventID,
		event.RequestID,
		linkID,
		event.ShortCode,
		event.OccurredAt.UTC(),
		event.IP,
		event.UserAgent,
		event.Referer,
	); err != nil {
		return fmt.Errorf("insert click event: %w", err)
	}

	return nil
}
