package app

import (
	"context"
	"testing"

	segmentkafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type testContextKey string

func TestExtractKafkaTraceContext_PreservesParentContext(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(prev)

	traceID, err := oteltrace.TraceIDFromHex("11111111111111111111111111111111")
	if err != nil {
		t.Fatalf("TraceIDFromHex error: %v", err)
	}
	spanID, err := oteltrace.SpanIDFromHex("2222222222222222")
	if err != nil {
		t.Fatalf("SpanIDFromHex error: %v", err)
	}

	parentSpanCtx := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
	})

	baseCtx, cancel := context.WithCancel(context.Background())
	baseCtx = context.WithValue(baseCtx, testContextKey("marker"), "present")

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(oteltrace.ContextWithSpanContext(context.Background(), parentSpanCtx), carrier)

	headers := make([]segmentkafka.Header, 0, len(carrier))
	for key, value := range carrier {
		headers = append(headers, segmentkafka.Header{Key: key, Value: []byte(value)})
	}

	derived := extractKafkaTraceContext(baseCtx, headers)
	cancel()

	if got := derived.Value(testContextKey("marker")); got != "present" {
		t.Fatalf("context value = %v, want present", got)
	}
	if err := derived.Err(); err != context.Canceled {
		t.Fatalf("ctx err = %v, want %v", err, context.Canceled)
	}

	gotSpanCtx := oteltrace.SpanContextFromContext(derived)
	if !gotSpanCtx.IsValid() {
		t.Fatal("expected extracted span context to be valid")
	}
	if gotSpanCtx.TraceID() != traceID {
		t.Fatalf("trace id = %s, want %s", gotSpanCtx.TraceID(), traceID)
	}
	if gotSpanCtx.SpanID() != spanID {
		t.Fatalf("span id = %s, want %s", gotSpanCtx.SpanID(), spanID)
	}
}
