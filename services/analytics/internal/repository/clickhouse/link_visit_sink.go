package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"sandbox_url_shortener/internal/platform/eventing"
	"sandbox_url_shortener/internal/platform/metrics"
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
	start := time.Now()
	ctx, span := otel.Tracer("analytics.clickhouse").Start(ctx, "analytics.clickhouse.insert_click_event")
	defer span.End()

	if event == nil {
		metrics.ObserveClickHouseOperation("analytics", "insert_click_event", "invalid_event", time.Since(start))
		span.SetStatus(codes.Error, "invalid_event")
		return fmt.Errorf("link.visited event is nil")
	}
	span.SetAttributes(
		attribute.String("event.id", event.EventID),
		attribute.String("link.id", event.LinkID),
		attribute.String("link.short_code", event.ShortCode),
	)

	eventID, err := uuid.Parse(strings.TrimSpace(event.EventID))
	if err != nil {
		metrics.ObserveClickHouseOperation("analytics", "insert_click_event", "invalid_event_id", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid_event_id")
		return fmt.Errorf("parse event_id: %w", err)
	}
	linkID, err := uuid.Parse(strings.TrimSpace(event.LinkID))
	if err != nil {
		metrics.ObserveClickHouseOperation("analytics", "insert_click_event", "invalid_link_id", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid_link_id")
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
		metrics.ObserveClickHouseOperation("analytics", "insert_click_event", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "insert_failed")
		return fmt.Errorf("insert click event: %w", err)
	}

	metrics.ObserveClickHouseOperation("analytics", "insert_click_event", "success", time.Since(start))
	span.SetStatus(codes.Ok, "")
	return nil
}
