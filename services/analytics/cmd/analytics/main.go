package main

import (
	"context"
	"log"

	"sandbox_url_shortener/services/analytics/internal/app"
)

func main() {
	ctx := context.Background()

	a, err := app.NewApp(ctx)
	if err != nil {
		log.Fatalf("failed to init analytics: %v", err)
	}

	a.Run(ctx)
}
