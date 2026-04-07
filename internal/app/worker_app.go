package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"sandbox_url_shortener/internal/config"
	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
)

// WorkerApp is the composition root for the worker binary.
// It exposes health and metrics only; Kafka consumer logic is added in E05.
type WorkerApp struct {
	cfg     *config.Config
	log     *slog.Logger
	srv     *http.Server
	closers []func() error
}

// NewWorkerApp initializes all dependencies and returns a ready-to-run WorkerApp.
func NewWorkerApp(ctx context.Context) (*WorkerApp, error) {
	w := &WorkerApp{}

	if err := w.initConfig(ctx); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	w.initLogger()
	w.initHTTPServer()

	return w, nil
}

// Run starts the admin HTTP server and blocks until a shutdown signal is received.
func (w *WorkerApp) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		w.log.Info("worker admin starting", "port", w.cfg.WorkerAdminPort)
		if err := w.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.log.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	w.log.Info("shutdown signal received")
	w.Shutdown(ctx)
}

// Shutdown gracefully drains the HTTP server and closes all resources in LIFO order.
func (w *WorkerApp) Shutdown(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if err := w.srv.Shutdown(ctx); err != nil {
		w.log.Error("http shutdown error", "error", err)
	}

	for i := len(w.closers) - 1; i >= 0; i-- {
		if err := w.closers[i](); err != nil {
			w.log.Error("closer error", "error", err)
		}
	}
}

func (w *WorkerApp) initConfig(ctx context.Context) error {
	cfg, err := config.Load(ctx)
	if err != nil {
		return err
	}
	w.cfg = cfg
	return nil
}

func (w *WorkerApp) initLogger() {
	w.log = logger.New(w.cfg.LogLevel, "worker", w.cfg.Env)
}

func (w *WorkerApp) initHTTPServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler()) // No infra deps yet; upgraded in E05.
	mux.Handle("GET /metrics", metrics.Handler())

	handler := middleware.Chain(w.log, mux)

	w.srv = &http.Server{
		Addr:         ":" + w.cfg.WorkerAdminPort,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
