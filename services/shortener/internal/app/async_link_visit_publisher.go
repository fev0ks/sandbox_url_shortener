package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"sandbox_url_shortener/internal/platform/eventing"
	"sandbox_url_shortener/internal/platform/metrics"
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
	queue  chan queuedLinkVisit
	wg     sync.WaitGroup
	once   sync.Once
	stopCh chan struct{}
}

type queuedLinkVisit struct {
	event       *eventing.LinkVisited
	spanContext oteltrace.SpanContext
}

func newAsyncLinkVisitPublisher(ctx context.Context, log *slog.Logger, base syncLinkVisitPublisher) *asyncLinkVisitPublisher {
	p := &asyncLinkVisitPublisher{
		log:    log,
		base:   base,
		queue:  make(chan queuedLinkVisit, linkVisitPublishQueueSize),
		stopCh: make(chan struct{}),
	}

	p.wg.Add(1)
	go p.run(context.WithoutCancel(ctx))

	return p
}

func (p *asyncLinkVisitPublisher) PublishLinkVisited(ctx context.Context, event *eventing.LinkVisited) error {
	if event == nil {
		return fmt.Errorf("link.visited event is nil")
	}

	select {
	case <-p.stopCh:
		return fmt.Errorf("async link publisher is closed")
	case p.queue <- queuedLinkVisit{
		event:       event,
		spanContext: oteltrace.SpanContextFromContext(ctx),
	}:
		return nil
	default:
		return fmt.Errorf("async link publisher queue is full")
	}
}

func (p *asyncLinkVisitPublisher) Close() error {
	var err error

	p.once.Do(func() {
		// Signal shutdown via stopCh only; do NOT close(p.queue) —
		// sending on a closed channel panics, and concurrent PublishLinkVisited
		// callers could hit that race.
		close(p.stopCh)
		p.wg.Wait()
		err = p.base.Close()
	})

	return err
}

func (p *asyncLinkVisitPublisher) publish(baseCtx context.Context, tracer oteltrace.Tracer, item queuedLinkVisit) {
	parentCtx := baseCtx
	if item.spanContext.IsValid() {
		parentCtx = oteltrace.ContextWithSpanContext(parentCtx, item.spanContext)
	}

	publishCtx, cancel := context.WithTimeout(parentCtx, linkVisitPublishTimeout)
	publishCtx, span := tracer.Start(publishCtx, "shortener.link.visited.publish")
	span.SetAttributes(
		attribute.String("event.id", item.event.EventID),
		attribute.String("link.id", item.event.LinkID),
		attribute.String("link.short_code", item.event.ShortCode),
	)

	err := p.base.PublishLinkVisited(publishCtx, item.event)
	cancel()
	if err != nil {
		metrics.ObserveLinkVisitPublish("shortener", "dispatch_failed")
		p.log.Warn("async link.visited publish failed",
			"operation", "publish_link_visited",
			"error_kind", "publish_failed",
			"error", err,
			"event_id", item.event.EventID,
			"link_id", item.event.LinkID,
			"short_code", item.event.ShortCode,
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish_failed")
		span.End()
		return
	}
	metrics.ObserveLinkVisitPublish("shortener", "dispatch_success")
	span.SetStatus(codes.Ok, "")
	span.End()
}

func (p *asyncLinkVisitPublisher) run(baseCtx context.Context) {
	defer p.wg.Done()

	tracer := otel.Tracer("shortener.async.publisher")

	for {
		select {
		case <-p.stopCh:
			p.drain(baseCtx, tracer)
			return
		case item := <-p.queue:
			p.publish(baseCtx, tracer, item)
		}
	}
}

func (p *asyncLinkVisitPublisher) drain(baseCtx context.Context, tracer oteltrace.Tracer) {
	for {
		select {
		case item := <-p.queue:
			p.publish(baseCtx, tracer, item)
		default:
			return
		}
	}
}
