// Command mcp-logs serves incident-investigation tools over MCP.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sandbox_url_shortener/services/mcp-logs/internal/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.NewApp(ctx)
	if err != nil {
		// The logger is not built yet and stdout belongs to the protocol,
		// so startup failures are reported on stderr directly.
		fmt.Fprintf(os.Stderr, "failed to init mcp-logs: %v\n", err)
		return 1
	}

	if err := a.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-logs failed: %v\n", err)
		return 1
	}
	return 0
}
