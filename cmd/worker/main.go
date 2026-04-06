package main

import (
	"context"
	"log"

	"sandbox_url_shortener/internal/app"
)

func main() {
	ctx := context.Background()

	w, err := app.NewWorkerApp(ctx)
	if err != nil {
		log.Fatalf("failed to init worker: %v", err)
	}

	w.Run(ctx)
}
