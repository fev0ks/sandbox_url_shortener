package service

import (
	"context"

	"sandbox_url_shortener/internal/platform/eventing"
)

// LinkVisitPublisher is the exact side-effect boundary the shortener service
// needs for best-effort redirect event publication.
type LinkVisitPublisher interface {
	PublishLinkVisited(ctx context.Context, event *eventing.LinkVisited) error
}
