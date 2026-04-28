package main

import (
	"context"
	"log"

	"sandbox_url_shortener/services/identity/internal/app"
)

func main() {
	ctx := context.Background()

	a, err := app.NewApp(ctx)
	if err != nil {
		log.Fatalf("failed to init identity: %v", err)
	}

	a.Run(ctx)
}
