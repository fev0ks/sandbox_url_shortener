// Package app provides the composition root for the shortener service binary.
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

	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/services/shortener/internal/config"
)

// App is the composition root for the shortener service.
type App struct {
	cfg     *config.Config
	log     *slog.Logger
	pool    *pgxpool.Pool
	di      *container
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
	a.initContainer()
	a.initHTTPServer()

	return a, nil
}

func (a *App) initContainer() {
	a.di = newContainer(a.cfg, a.log, a.pool)
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		a.log.Info("shortener starting", "port", a.cfg.AppPort)
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
	a.log = logger.New(a.cfg.LogLevel, "shortener", a.cfg.Env)
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
	a.srv = &http.Server{
		Addr:         ":" + a.cfg.AppPort,
		Handler:      a.buildRouter(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
