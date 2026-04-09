package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/services/analytics/internal/config"
)

// App is the composition root for the analytics binary.
type App struct {
	cfg       *config.Config
	log       *slog.Logger
	srv       *http.Server
	closers   []func() error
	consumers []Consumer
}

// NewApp initializes all dependencies and returns a ready-to-run analytics App.
func NewApp(ctx context.Context) (*App, error) {
	a := &App{}

	if err := a.initConfig(ctx); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	a.initLogger()
	if err := a.initConsumers(ctx); err != nil {
		return nil, fmt.Errorf("init consumers: %w", err)
	}
	a.initHTTPServer()

	return a, nil
}

// Run starts the admin HTTP server and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a.startConsumers(ctx)

	go func() {
		a.log.Info("analytics admin starting", "port", a.cfg.AdminPort)
		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	a.log.Info("shutdown signal received")
	a.Shutdown(ctx)
}

// Shutdown gracefully drains the HTTP server and closes all resources in LIFO order.
func (a *App) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.log.Error("http shutdown error", "error", err)
	}

	a.waitConsumers(ctx)

	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i](); err != nil {
			a.log.Error("closer error", "error", err)
		}
	}
}

func (a *App) initConfig(ctx context.Context) error {
	cfg, err := config.Load(ctx)
	if err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

func (a *App) initLogger() {
	a.log = logger.New(a.cfg.LogLevel, "analytics", a.cfg.Env)
}

func (a *App) initConsumers(ctx context.Context) error {
	linkVisited, err := newLinkVisitedConsumer(ctx, a.cfg, a.log)
	if err != nil {
		return err
	}
	if linkVisited != nil {
		a.consumers = append(a.consumers, linkVisited)
		a.addCloser(linkVisited.Close)
	}
	return nil
}

func (a *App) startConsumers(ctx context.Context) {
	for _, consumer := range a.consumers {
		consumer.Start(ctx)
	}
}

func (a *App) initHTTPServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler(a.consumersReady))
	mux.Handle("GET /metrics", metrics.Handler())

	handler := middleware.Compose(mux,
		middleware.Recovery(a.log),
		middleware.RequestID,
		middleware.CorrelationID,
		middleware.Logging(a.log),
	)

	a.srv = &http.Server{
		Addr:         ":" + a.cfg.AdminPort,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (a *App) addCloser(fn func() error) {
	a.closers = append(a.closers, fn)
}

func (a *App) consumersReady(ctx context.Context) error {
	for _, consumer := range a.consumers {
		if err := consumer.Ready(ctx); err != nil {
			return fmt.Errorf("%s consumer not ready: %w", consumer.Name(), err)
		}
	}
	return nil
}

func (a *App) waitConsumers(ctx context.Context) {
	for _, consumer := range a.consumers {
		done := consumer.Done()
		if done == nil {
			continue
		}
		select {
		case <-done:
		case <-ctx.Done():
			a.log.Warn("analytics consumer shutdown timed out", "consumer", consumer.Name())
			return
		}
	}
}
