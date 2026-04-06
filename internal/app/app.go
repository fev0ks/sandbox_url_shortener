// Package app provides the composition root for the api-http-stdlib binary.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/internal/config"
	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/repository/postgres"
	"sandbox_url_shortener/internal/service"
	"sandbox_url_shortener/internal/transport/http/stdlib/handler"
)

// App is the composition root for the api-http-stdlib service.
type App struct {
	cfg     *config.Config
	log     *slog.Logger
	pool    *pgxpool.Pool
	srv     *http.Server
	closers []func() error
}

// NewApp initializes all dependencies in order and returns a ready-to-run App.
func NewApp(ctx context.Context) (*App, error) {
	a := &App{}

	if err := a.initConfig(ctx); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	a.initLogger()

	if err := a.initDB(ctx); err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}
	a.initHTTPServer()

	return a, nil
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		a.log.Info("api-http-stdlib starting", "port", a.cfg.AppPort)
		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error("server error", "error", err)
		}
	}()

	<-ctx.Done()
	a.log.Info("shutdown signal received")
	a.Shutdown()
}

// Shutdown gracefully drains the HTTP server and closes all resources in LIFO order.
func (a *App) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

func (a *App) addCloser(fn func() error) {
	a.closers = append(a.closers, fn)
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
	a.log = logger.New(a.cfg.LogLevel, "api-http-stdlib", a.cfg.Env)
}

func (a *App) initDB(ctx context.Context) error {
	poolCfg, err := pgxpool.ParseConfig(a.cfg.DBDSN)
	if err != nil {
		return fmt.Errorf("parse DB_DSN: %w", err)
	}
	poolCfg.MaxConns = a.cfg.DBMaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}

	a.pool = pool
	a.addCloser(func() error { pool.Close(); return nil })
	return nil
}

func (a *App) initHTTPServer() {
	// Repository → Service → Handler wiring.
	linkRepo := postgres.NewLinkRepo(a.pool)
	linkSvc := service.NewLinkService(linkRepo, a.log)
	linkH := handler.NewLinkHandler(linkSvc, a.log, a.cfg.BaseURL)
	redirectH := handler.NewRedirectHandler(linkSvc, a.log)

	mux := http.NewServeMux()

	// Platform routes.
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler(func(ctx context.Context) error {
		return a.pool.Ping(ctx)
	}))
	mux.Handle("GET /metrics", metrics.Handler())

	// Link CRUD API.
	mux.HandleFunc("POST /api/v1/links", linkH.Create)
	mux.HandleFunc("GET /api/v1/links/{id}", linkH.GetByID)
	mux.HandleFunc("POST /api/v1/links/{id}/disable", linkH.Disable)
	mux.HandleFunc("POST /api/v1/links/{id}/enable", linkH.Enable)
	mux.HandleFunc("DELETE /api/v1/links/{id}", linkH.Delete)

	// Redirect (catch-all, registered last).
	mux.HandleFunc("GET /{shortCode}", redirectH.Redirect)

	chainedHandler := middleware.Chain(a.log, mux)

	a.srv = &http.Server{
		Addr:         ":" + a.cfg.AppPort,
		Handler:      chainedHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
