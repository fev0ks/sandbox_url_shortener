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
// It exposes health and metrics only; event consumer logic is added in E05.
type App struct {
	cfg     *config.Config
	log     *slog.Logger
	srv     *http.Server
	closers []func() error
}

// NewApp initializes all dependencies and returns a ready-to-run analytics App.
func NewApp(ctx context.Context) (*App, error) {
	a := &App{}

	if err := a.initConfig(ctx); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	a.initLogger()
	a.initHTTPServer()

	return a, nil
}

// Run starts the admin HTTP server and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

func (a *App) initHTTPServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler()) // No infra deps yet; upgraded in E05.
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
