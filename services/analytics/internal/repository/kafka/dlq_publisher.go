package kafka

import (
	"context"
	"fmt"
	"strconv"

	segmentkafka "github.com/segmentio/kafka-go"
)

const (
	headerDLQReason       = "dlq_reason"
	headerDLQConsumer     = "dlq_consumer"
	headerDLQAttempts     = "dlq_attempts"
	headerDLQError        = "dlq_error"
	headerSourceTopic     = "source_topic"
	headerSourcePartition = "source_partition"
	headerSourceOffset    = "source_offset"
)

// LinkVisitDLQPublisher forwards failed main-topic messages into the dedicated DLQ topic.
type LinkVisitDLQPublisher struct {
	writer *segmentkafka.Writer
}

func NewLinkVisitDLQPublisher(brokers []string, topic string) *LinkVisitDLQPublisher {
	return &LinkVisitDLQPublisher{
		writer: &segmentkafka.Writer{
			Addr:     segmentkafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &segmentkafka.Hash{},
		},
	}
}

func (p *LinkVisitDLQPublisher) Publish(ctx context.Context, msg segmentkafka.Message, reason, consumer string, attempts int, processErr error) error {
	headers := append([]segmentkafka.Header{}, msg.Headers...)
	headers = append(headers,
		segmentkafka.Header{Key: headerDLQReason, Value: []byte(reason)},
		segmentkafka.Header{Key: headerDLQConsumer, Value: []byte(consumer)},
		segmentkafka.Header{Key: headerDLQAttempts, Value: []byte(strconv.Itoa(attempts))},
		segmentkafka.Header{Key: headerSourceTopic, Value: []byte(msg.Topic)},
		segmentkafka.Header{Key: headerSourcePartition, Value: []byte(strconv.Itoa(msg.Partition))},
		segmentkafka.Header{Key: headerSourceOffset, Value: []byte(strconv.FormatInt(msg.Offset, 10))},
	)
	if processErr != nil {
		headers = append(headers, segmentkafka.Header{Key: headerDLQError, Value: []byte(processErr.Error())})
	}

	if err := p.writer.WriteMessages(ctx, segmentkafka.Message{
		Key:     msg.Key,
		Value:   msg.Value,
		Time:    msg.Time,
		Headers: headers,
	}); err != nil {
		return fmt.Errorf("publish DLQ message: %w", err)
	}

	return nil
}

func (p *LinkVisitDLQPublisher) Close() error {
	return p.writer.Close()
}
