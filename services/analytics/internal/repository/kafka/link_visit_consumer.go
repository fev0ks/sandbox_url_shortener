package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	segmentkafka "github.com/segmentio/kafka-go"

	"sandbox_url_shortener/internal/platform/eventing"
)

// InvalidMessageError signals that a fetched Kafka record cannot be decoded into the shared event contract.
type InvalidMessageError struct {
	Message segmentkafka.Message
	Err     error
}

func (e *InvalidMessageError) Error() string {
	return fmt.Sprintf("invalid Kafka message: %v", e.Err)
}

func (e *InvalidMessageError) Unwrap() error {
	return e.Err
}

// LinkVisitConsumer fetches link.visited events from Kafka.
type LinkVisitConsumer struct {
	reader  *segmentkafka.Reader
	brokers []string
}

func NewLinkVisitConsumer(brokers []string, topic, groupID string) *LinkVisitConsumer {
	return &LinkVisitConsumer{
		reader: segmentkafka.NewReader(segmentkafka.ReaderConfig{
			Brokers: brokers,
			GroupID: groupID,
			Topic:   topic,
			MaxWait: 500 * time.Millisecond,
		}),
		brokers: brokers,
	}
}

func (c *LinkVisitConsumer) FetchLinkVisited(ctx context.Context) (*eventing.LinkVisited, segmentkafka.Message, error) {
	msg, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return nil, segmentkafka.Message{}, fmt.Errorf("fetch link.visited message: %w", err)
	}

	event, err := eventing.UnmarshalLinkVisited(msg.Value)
	if err != nil {
		return nil, msg, &InvalidMessageError{
			Message: msg,
			Err:     fmt.Errorf("decode link.visited: %w", err),
		}
	}

	return event, msg, nil
}

func (c *LinkVisitConsumer) CommitMessage(ctx context.Context, msg segmentkafka.Message) error {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		return fmt.Errorf("commit Kafka message: %w", err)
	}
	return nil
}

func (c *LinkVisitConsumer) Check(ctx context.Context) error {
	if len(c.brokers) == 0 {
		return errors.New("no Kafka brokers configured")
	}

	broker := strings.TrimSpace(c.brokers[0])
	if broker == "" {
		return errors.New("first Kafka broker is empty")
	}

	conn, err := segmentkafka.DialContext(ctx, "tcp", broker)
	if err != nil {
		return fmt.Errorf("dial Kafka broker %s: %w", broker, err)
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("close Kafka broker connection: %w", err)
	}

	return nil
}

func (c *LinkVisitConsumer) Close() error {
	return c.reader.Close()
}
