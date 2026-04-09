package kafka

import (
	"context"
	"fmt"

	segmentkafka "github.com/segmentio/kafka-go"

	"sandbox_url_shortener/internal/platform/eventing"
)

// LinkVisitPublisher publishes link.visited events to Kafka.
type LinkVisitPublisher struct {
	writer *segmentkafka.Writer
}

func NewLinkVisitPublisher(brokers []string, topic string) *LinkVisitPublisher {
	return &LinkVisitPublisher{
		writer: &segmentkafka.Writer{
			Addr:     segmentkafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &segmentkafka.Hash{},
		},
	}
}

func (p *LinkVisitPublisher) PublishLinkVisited(ctx context.Context, event *eventing.LinkVisited) error {
	payload, err := eventing.MarshalLinkVisited(event)
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx, segmentkafka.Message{
		Key:   []byte(event.LinkID),
		Value: payload,
		Time:  event.OccurredAt,
		Headers: []segmentkafka.Header{
			{Key: "event_name", Value: []byte(event.EventName)},
			{Key: "event_version", Value: []byte(event.Version)},
			{Key: "request_id", Value: []byte(event.RequestID)},
		},
	})
	if err != nil {
		return fmt.Errorf("publish link.visited: %w", err)
	}
	return nil
}

func (p *LinkVisitPublisher) Close() error {
	return p.writer.Close()
}
