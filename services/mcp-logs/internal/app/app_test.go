package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestHTTPStartsAndStopsWithUnavailableUpstreams(t *testing.T) {
	t.Setenv("MCP_LOGS_TRANSPORT", "http")
	t.Setenv("MCP_LOGS_HTTP_ADDRESS", "127.0.0.1:0")
	t.Setenv("MCP_LOGS_ELASTICSEARCH_URL", "http://127.0.0.1:1")
	t.Setenv("MCP_LOGS_TEMPO_URL", "http://127.0.0.1:1")
	t.Setenv("MCP_LOGS_POSTGRES_DSN", "postgres://reader:unused@127.0.0.1:1/missing?sslmode=disable")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := NewApp(ctx)
	if err != nil {
		t.Fatalf("unavailable upstream prevented startup: %v", err)
	}
	if a.httpHandler == nil || a.database == nil {
		t.Fatal("HTTP/database dependencies not wired")
	}
	cancel()
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP shutdown exceeded deadline")
	}
}

func TestInvalidDSNDoesNotLeakConnectionDetails(t *testing.T) {
	t.Setenv("MCP_LOGS_POSTGRES_DSN", "postgres://reader:secret-password@[invalid/missing")
	_, err := NewApp(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("unsafe configuration failure: %v", err)
	}
}
