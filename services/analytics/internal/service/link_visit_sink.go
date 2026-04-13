package service

import (
	"context"
	"fmt"
	"log/slog"

	"sandbox_url_shortener/internal/platform/eventing"
)

// LinkVisitSink is the analytics-side boundary for handling decoded link.visited events.
type LinkVisitSink interface {
	HandleLinkVisited(ctx context.Context, event *eventing.LinkVisited) error
}

// LoggingLinkVisitSink is a bootstrap sink that only logs consumed events.
// E06 replaces it with persistent analytics storage.
type LoggingLinkVisitSink struct {
	log *slog.Logger
}

func NewLoggingLinkVisitSink(log *slog.Logger) *LoggingLinkVisitSink {
	return &LoggingLinkVisitSink{log: log}
}

func (s *LoggingLinkVisitSink) HandleLinkVisited(ctx context.Context, event *eventing.LinkVisited) error {
	if event == nil {
		return fmt.Errorf("link.visited event is nil")
	}

	s.log.DebugContext(ctx, "analytics link.visited consumed",
		"operation", "clickhouse_write",
		"event_id", event.EventID,
		"link_id", event.LinkID,
		"short_code", event.ShortCode,
		"occurred_at", event.OccurredAt,
	)

	return nil
}
