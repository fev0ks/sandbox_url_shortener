package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	segmentkafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"

	"sandbox_url_shortener/internal/platform/eventing"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/services/analytics/internal/config"
	kafkarepo "sandbox_url_shortener/services/analytics/internal/repository/kafka"
	"sandbox_url_shortener/services/analytics/internal/service"
)

// Consumer is a long-running analytics runtime job such as a Kafka consumer.
type Consumer interface {
	Name() string
	Start(ctx context.Context)
	Ready(ctx context.Context) error
	Done() <-chan struct{}
	Close() error
}

const (
	linkVisitedMaxAttempts  = 3
	linkVisitedRetryBackoff = 500 * time.Millisecond
	dlqReasonInvalidPayload = "invalid_payload"
	dlqReasonSinkFailure    = "sink_failure"
)

type linkVisitMessageRepo interface {
	FetchLinkVisited(ctx context.Context) (*eventing.LinkVisited, segmentkafka.Message, error)
	CommitMessage(ctx context.Context, msg segmentkafka.Message) error
	Check(ctx context.Context) error
	Close() error
}

type linkVisitDLQPublisher interface {
	Publish(ctx context.Context, msg segmentkafka.Message, reason, consumer string, attempts int, processErr error) error
	Close() error
}

type linkVisitedConsumer struct {
	log   *slog.Logger
	cfg   *config.Config
	repo  linkVisitMessageRepo
	dlq   linkVisitDLQPublisher
	sink  service.LinkVisitSink
	done  chan struct{}
	state struct {
		mu      sync.RWMutex
		started bool
		loopErr error
	}
}

func newLinkVisitedConsumerWithSink(ctx context.Context, cfg *config.Config, log *slog.Logger, sink service.LinkVisitSink) (*linkVisitedConsumer, error) {
	if !cfg.KafkaConsumerEnabled {
		return nil, nil
	}

	repo := kafkarepo.NewLinkVisitConsumer(splitCSV(cfg.KafkaBrokers), cfg.KafkaTopicLinkVisited, cfg.KafkaConsumerGroup)
	dlq := kafkarepo.NewLinkVisitDLQPublisher(splitCSV(cfg.KafkaBrokers), cfg.KafkaTopicLinkVisitedDLQ)

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := repo.Check(checkCtx); err != nil {
		if closeErr := repo.Close(); closeErr != nil {
			log.Warn("analytics consumer close after failed check", "error", closeErr)
		}
		if closeErr := dlq.Close(); closeErr != nil {
			log.Warn("analytics DLQ publisher close after failed check", "error", closeErr)
		}
		return nil, err
	}

	return &linkVisitedConsumer{
		log:  log,
		cfg:  cfg,
		repo: repo,
		dlq:  dlq,
		sink: sink,
		done: make(chan struct{}),
	}, nil
}

func (c *linkVisitedConsumer) Name() string {
	return "link.visited"
}

func (c *linkVisitedConsumer) Start(ctx context.Context) {
	if c == nil {
		return
	}

	c.setStarted(true)

	go func() {
		defer close(c.done)
		defer c.setStarted(false)

		c.log.Info("analytics consumer starting",
			"operation", "consumer_start",
			"consumer", c.Name(),
			"topic", c.cfg.KafkaTopicLinkVisited,
			"group", c.cfg.KafkaConsumerGroup,
		)

		for {
			event, msg, err := c.repo.FetchLinkVisited(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					c.clearLoopError()
					return
				}

				var invalidErr *kafkarepo.InvalidMessageError
				if errors.As(err, &invalidErr) {
					metrics.ObserveAnalyticsConsumer("analytics", "fetch", "invalid_payload")
					c.log.Warn("analytics invalid link.visited payload",
						"operation", "consumer_fetch",
						"error_kind", dlqReasonInvalidPayload,
						"consumer", c.Name(),
						"error", invalidErr.Err,
						"topic", invalidErr.Message.Topic,
						"partition", invalidErr.Message.Partition,
						"offset", invalidErr.Message.Offset,
					)
					if handleErr := c.handleInvalidMessage(ctx, invalidErr); handleErr != nil {
						c.setLoopError(handleErr)
						c.log.Error("analytics invalid message handling failed",
							"operation", "consumer_process",
							"error_kind", "invalid_message_handling_failed",
							"consumer", c.Name(),
							"error", handleErr,
						)
						return
					}
					continue
				}

				metrics.ObserveAnalyticsConsumer("analytics", "fetch", "failure")
				c.setLoopError(err)
				c.log.Error("analytics consumer fetch failed",
					"operation", "consumer_fetch",
					"error_kind", "fetch_failed",
					"consumer", c.Name(),
					"error", err,
				)
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					c.clearLoopError()
					return
				}
				continue
			}

			c.clearLoopError()

			if err := c.processMessage(ctx, event, msg); err != nil {
				metrics.ObserveAnalyticsConsumer("analytics", "process", "failure")
				c.setLoopError(err)
				c.log.Error("analytics message processing failed",
					"operation", "consumer_process",
					"error_kind", "process_failed",
					"consumer", c.Name(),
					"error", err,
				)
				return
			}
		}
	}()
}

func (c *linkVisitedConsumer) Ready(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if !c.isStarted() {
		return fmt.Errorf("%s consumer loop has not started", c.Name())
	}
	if err := c.currentLoopError(); err != nil {
		return err
	}
	if err := c.repo.Check(ctx); err != nil {
		return err
	}
	return nil
}

func (c *linkVisitedConsumer) Done() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

func (c *linkVisitedConsumer) Close() error {
	if c == nil {
		return nil
	}
	if err := c.repo.Close(); err != nil {
		return err
	}
	if c.dlq != nil {
		return c.dlq.Close()
	}
	return nil
}

func (c *linkVisitedConsumer) commitMessage(ctx context.Context, msg segmentkafka.Message) error {
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return c.repo.CommitMessage(commitCtx, msg)
}

func (c *linkVisitedConsumer) processMessage(ctx context.Context, event *eventing.LinkVisited, msg segmentkafka.Message) error {
	procCtx := extractKafkaTraceContext(ctx, msg.Headers)
	procCtx, span := otel.Tracer("analytics.consumer").Start(procCtx, "analytics.link.visited.process")
	defer span.End()

	span.SetAttributes(
		attribute.String("event.id", event.EventID),
		attribute.String("link.id", event.LinkID),
		attribute.String("link.short_code", event.ShortCode),
	)

	var lastErr error

	for attempt := 1; attempt <= linkVisitedMaxAttempts; attempt++ {
		if err := c.sink.HandleLinkVisited(procCtx, event); err != nil {
			lastErr = err
			c.log.Warn("analytics sink attempt failed",
				"operation", "clickhouse_write",
				"error_kind", dlqReasonSinkFailure,
				"consumer", c.Name(),
				"attempt", attempt,
				"max_attempts", linkVisitedMaxAttempts,
				"event_id", event.EventID,
				"link_id", event.LinkID,
				"error", err,
			)
			if attempt < linkVisitedMaxAttempts {
				select {
				case <-time.After(linkVisitedRetryBackoff):
				case <-ctx.Done():
					span.RecordError(ctx.Err())
					span.SetStatus(codes.Error, "context_canceled")
					return ctx.Err()
				}
				continue
			}

			if err := c.publishDLQ(procCtx, msg, dlqReasonSinkFailure, attempt, lastErr); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "dlq_publish_failed")
				return fmt.Errorf("publish sink failure to DLQ: %w", err)
			}
			if err := c.commitMessage(ctx, msg); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "commit_failed")
				return fmt.Errorf("commit sink failure message: %w", err)
			}
			metrics.ObserveAnalyticsConsumer("analytics", "process", "dlq")
			span.SetStatus(codes.Ok, "")
			return nil
		}

		if err := c.commitMessage(ctx, msg); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "commit_failed")
			return fmt.Errorf("commit processed message: %w", err)
		}
		metrics.ObserveAnalyticsConsumer("analytics", "process", "success")
		span.SetStatus(codes.Ok, "")
		return nil
	}

	span.RecordError(lastErr)
	span.SetStatus(codes.Error, "process_failed")
	return lastErr
}

func (c *linkVisitedConsumer) handleInvalidMessage(ctx context.Context, invalidErr *kafkarepo.InvalidMessageError) error {
	if invalidErr == nil {
		return fmt.Errorf("invalid message error is nil")
	}
	procCtx := extractKafkaTraceContext(ctx, invalidErr.Message.Headers)
	procCtx, span := otel.Tracer("analytics.consumer").Start(procCtx, "analytics.link.visited.invalid_payload")
	defer span.End()

	span.SetAttributes(
		attribute.String("error.kind", dlqReasonInvalidPayload),
		attribute.String("kafka.topic", invalidErr.Message.Topic),
		attribute.Int64("kafka.offset", invalidErr.Message.Offset),
	)

	if err := c.publishDLQ(procCtx, invalidErr.Message, dlqReasonInvalidPayload, 1, invalidErr.Err); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "dlq_publish_failed")
		return fmt.Errorf("publish invalid payload to DLQ: %w", err)
	}
	if err := c.commitMessage(ctx, invalidErr.Message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "commit_failed")
		return fmt.Errorf("commit invalid payload message: %w", err)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func (c *linkVisitedConsumer) publishDLQ(ctx context.Context, msg segmentkafka.Message, reason string, attempts int, processErr error) error {
	if c.dlq == nil {
		return fmt.Errorf("DLQ publisher is not configured")
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	publishCtx, span := otel.Tracer("analytics.consumer").Start(publishCtx, "analytics.link.visited.dlq_publish")
	defer span.End()

	span.SetAttributes(
		attribute.String("error.kind", reason),
		attribute.String("kafka.topic", msg.Topic),
		attribute.Int("kafka.partition", msg.Partition),
		attribute.Int64("kafka.offset", msg.Offset),
		attribute.Int("dlq.attempts", attempts),
	)

	if err := c.dlq.Publish(publishCtx, msg, reason, c.Name(), attempts, processErr); err != nil {
		metrics.ObserveAnalyticsConsumer("analytics", "dlq_publish", "failure")
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish_failed")
		return err
	}
	metrics.ObserveAnalyticsConsumer("analytics", "dlq_publish", "success")

	c.log.Warn("analytics message forwarded to DLQ",
		"operation", "dlq_publish",
		"consumer", c.Name(),
		"reason", reason,
		"attempts", attempts,
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)
	span.SetStatus(codes.Ok, "")

	return nil
}

func extractKafkaTraceContext(ctx context.Context, headers []segmentkafka.Header) context.Context {
	carrier := propagation.MapCarrier{}
	for _, header := range headers {
		carrier[header.Key] = string(header.Value)
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

func (c *linkVisitedConsumer) setStarted(started bool) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.started = started
}

func (c *linkVisitedConsumer) isStarted() bool {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.state.started
}

func (c *linkVisitedConsumer) setLoopError(err error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	c.state.loopErr = err
}

func (c *linkVisitedConsumer) clearLoopError() {
	c.setLoopError(nil)
}

func (c *linkVisitedConsumer) currentLoopError() error {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	return c.state.loopErr
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
