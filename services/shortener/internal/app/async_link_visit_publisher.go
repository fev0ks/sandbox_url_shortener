package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"sandbox_url_shortener/internal/platform/eventing"
	"sandbox_url_shortener/services/shortener/internal/service"
)

const (
	linkVisitPublishQueueSize = 256
	linkVisitPublishTimeout   = 2 * time.Second
)

type syncLinkVisitPublisher interface {
	service.LinkVisitPublisher
	Close() error
}

type asyncLinkVisitPublisher struct {
	log    *slog.Logger
	base   syncLinkVisitPublisher
	queue  chan *eventing.LinkVisited
	wg     sync.WaitGroup
	once   sync.Once
	stopCh chan struct{}
}

func newAsyncLinkVisitPublisher(log *slog.Logger, base syncLinkVisitPublisher) *asyncLinkVisitPublisher {
	p := &asyncLinkVisitPublisher{
		log:    log,
		base:   base,
		queue:  make(chan *eventing.LinkVisited, linkVisitPublishQueueSize),
		stopCh: make(chan struct{}),
	}

	p.wg.Add(1)
	go p.run()

	return p
}

func (p *asyncLinkVisitPublisher) PublishLinkVisited(_ context.Context, event *eventing.LinkVisited) error {
	if event == nil {
		return fmt.Errorf("link.visited event is nil")
	}

	select {
	case <-p.stopCh:
		return fmt.Errorf("async link publisher is closed")
	case p.queue <- event:
		return nil
	default:
		return fmt.Errorf("async link publisher queue is full")
	}
}

func (p *asyncLinkVisitPublisher) Close() error {
	var err error

	p.once.Do(func() {
		close(p.stopCh)
		close(p.queue)
		p.wg.Wait()
		err = p.base.Close()
	})

	return err
}

func (p *asyncLinkVisitPublisher) run() {
	defer p.wg.Done()

	for event := range p.queue {
		publishCtx, cancel := context.WithTimeout(context.Background(), linkVisitPublishTimeout)
		err := p.base.PublishLinkVisited(publishCtx, event)
		cancel()
		if err != nil {
			p.log.Warn("async link.visited publish failed",
				"operation", "publish_link_visited",
				"error_kind", "publish_failed",
				"error", err,
				"event_id", event.EventID,
				"link_id", event.LinkID,
				"short_code", event.ShortCode,
			)
		}
	}
}
