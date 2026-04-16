package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	segmentkafka "github.com/segmentio/kafka-go"

	"sandbox_url_shortener/internal/platform/eventing"
	kafkarepo "sandbox_url_shortener/services/analytics/internal/repository/kafka"
)

type fakeRepo struct {
	commitCalls int
}

func (f *fakeRepo) FetchLinkVisited(context.Context) (*eventing.LinkVisited, segmentkafka.Message, error) {
	return nil, segmentkafka.Message{}, errors.New("not used")
}

func (f *fakeRepo) CommitMessage(context.Context, segmentkafka.Message) error {
	f.commitCalls++
	return nil
}

func (f *fakeRepo) Check(context.Context) error { return nil }
func (f *fakeRepo) Close() error                { return nil }

type fakeDLQ struct {
	publishCalls int
}

func (f *fakeDLQ) Publish(context.Context, segmentkafka.Message, string, string, int, error) error {
	f.publishCalls++
	return nil
}

func (f *fakeDLQ) Close() error { return nil }

type fakeFailingSink struct {
	calls int
	err   error
}

func (f *fakeFailingSink) HandleLinkVisited(context.Context, *eventing.LinkVisited) error {
	f.calls++
	return f.err
}

func TestLinkVisitedConsumer_ProcessMessage_RetriesThenDLQ(t *testing.T) {
	repo := &fakeRepo{}
	dlq := &fakeDLQ{}
	sink := &fakeFailingSink{err: errors.New("sink failed")}
	consumer := &linkVisitedConsumer{
		log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		repo: repo,
		dlq:  dlq,
		sink: sink,
		cfg:  nil,
		done: make(chan struct{}),
	}

	event := &eventing.LinkVisited{
		EventName:  eventing.LinkVisitedEventName,
		Version:    eventing.LinkVisitedVersion,
		EventID:    "11111111-1111-1111-1111-111111111111",
		RequestID:  "request-1",
		LinkID:     "22222222-2222-2222-2222-222222222222",
		ShortCode:  "retry01",
		OccurredAt: time.Now().UTC(),
		IP:         "127.0.0.1",
	}
	msg := segmentkafka.Message{Topic: "link.visited.v1"}

	if err := consumer.processMessage(context.Background(), event, msg); err != nil {
		t.Fatalf("processMessage error: %v", err)
	}
	if sink.calls != linkVisitedMaxAttempts {
		t.Fatalf("sink calls = %d, want %d", sink.calls, linkVisitedMaxAttempts)
	}
	if dlq.publishCalls != 1 {
		t.Fatalf("DLQ publish calls = %d, want 1", dlq.publishCalls)
	}
	if repo.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", repo.commitCalls)
	}
}

func TestLinkVisitedConsumer_HandleInvalidMessage_PublishesDLQAndCommits(t *testing.T) {
	repo := &fakeRepo{}
	dlq := &fakeDLQ{}
	consumer := &linkVisitedConsumer{
		log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		repo: repo,
		dlq:  dlq,
		done: make(chan struct{}),
	}

	invalidErr := &kafkarepo.InvalidMessageError{
		Message: segmentkafka.Message{Topic: "link.visited.v1"},
		Err:     errors.New("bad payload"),
	}

	if err := consumer.handleInvalidMessage(context.Background(), invalidErr); err != nil {
		t.Fatalf("handleInvalidMessage error: %v", err)
	}
	if dlq.publishCalls != 1 {
		t.Fatalf("DLQ publish calls = %d, want 1", dlq.publishCalls)
	}
	if repo.commitCalls != 1 {
		t.Fatalf("commit calls = %d, want 1", repo.commitCalls)
	}
}
