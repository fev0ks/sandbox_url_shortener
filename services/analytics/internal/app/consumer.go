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

	"sandbox_url_shortener/internal/platform/eventing"
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

func newLinkVisitedConsumer(ctx context.Context, cfg *config.Config, log *slog.Logger) (*linkVisitedConsumer, error) {
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
		return nil, err
	}

	return &linkVisitedConsumer{
		log:  log,
		cfg:  cfg,
		repo: repo,
		dlq:  dlq,
		sink: service.NewLoggingLinkVisitSink(log),
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
					c.log.Warn("analytics invalid link.visited payload",
						"consumer", c.Name(),
						"error", invalidErr.Err,
						"topic", invalidErr.Message.Topic,
						"partition", invalidErr.Message.Partition,
						"offset", invalidErr.Message.Offset,
					)
					if handleErr := c.handleInvalidMessage(ctx, invalidErr); handleErr != nil {
						c.setLoopError(handleErr)
						c.log.Error("analytics invalid message handling failed",
							"consumer", c.Name(),
							"error", handleErr,
						)
						return
					}
					continue
				}

				c.setLoopError(err)
				c.log.Error("analytics consumer fetch failed",
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
				c.setLoopError(err)
				c.log.Error("analytics message processing failed",
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
	var lastErr error

	for attempt := 1; attempt <= linkVisitedMaxAttempts; attempt++ {
		if err := c.sink.HandleLinkVisited(ctx, event); err != nil {
			lastErr = err
			c.log.Warn("analytics sink attempt failed",
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
					return ctx.Err()
				}
				continue
			}

			if err := c.publishDLQ(ctx, msg, dlqReasonSinkFailure, attempt, lastErr); err != nil {
				return fmt.Errorf("publish sink failure to DLQ: %w", err)
			}
			if err := c.commitMessage(ctx, msg); err != nil {
				return fmt.Errorf("commit sink failure message: %w", err)
			}
			return nil
		}

		if err := c.commitMessage(ctx, msg); err != nil {
			return fmt.Errorf("commit processed message: %w", err)
		}
		return nil
	}

	return lastErr
}

func (c *linkVisitedConsumer) handleInvalidMessage(ctx context.Context, invalidErr *kafkarepo.InvalidMessageError) error {
	if invalidErr == nil {
		return fmt.Errorf("invalid message error is nil")
	}
	if err := c.publishDLQ(ctx, invalidErr.Message, dlqReasonInvalidPayload, 1, invalidErr.Err); err != nil {
		return fmt.Errorf("publish invalid payload to DLQ: %w", err)
	}
	if err := c.commitMessage(ctx, invalidErr.Message); err != nil {
		return fmt.Errorf("commit invalid payload message: %w", err)
	}
	return nil
}

func (c *linkVisitedConsumer) publishDLQ(ctx context.Context, msg segmentkafka.Message, reason string, attempts int, processErr error) error {
	if c.dlq == nil {
		return fmt.Errorf("DLQ publisher is not configured")
	}

	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if err := c.dlq.Publish(publishCtx, msg, reason, c.Name(), attempts, processErr); err != nil {
		return err
	}

	c.log.Warn("analytics message forwarded to DLQ",
		"consumer", c.Name(),
		"reason", reason,
		"attempts", attempts,
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
	)

	return nil
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
