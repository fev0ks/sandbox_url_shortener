// Package app provides the composition root for the identity service binary.
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

	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/platform/telemetry"
	"sandbox_url_shortener/services/identity/internal/config"
)

const (
	corsHeaderAllowCredentials = "Access-Control-Allow-Credentials"
	corsHeaderAllowHeaders     = "Access-Control-Allow-Headers"
	corsHeaderAllowMethods     = "Access-Control-Allow-Methods"
	corsHeaderAllowOrigin      = "Access-Control-Allow-Origin"
	corsHeaderVary             = "Vary"

	corsAllowCredentialsValue = "true"
	corsAllowHeadersValue     = "Content-Type, X-CSRF-Token, X-Request-ID"
	corsAllowMethodsValue     = "GET, POST, DELETE, OPTIONS"
	corsVaryOriginValue       = "Origin"
)

// App is the composition root for the identity service.
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
	if err := a.initTelemetry(ctx); err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}
	if err := a.initDB(ctx); err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}
	a.initContainer()
	a.initHTTPServer()
	return a, nil
}

// Run starts the HTTP server and blocks until a shutdown signal is received.
func (a *App) Run(ctx context.Context) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		a.log.Info("identity starting", "port", a.cfg.AppPort)
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
	a.log = logger.New(a.cfg.LogLevel, "identity", a.cfg.Env)
}

func (a *App) initTelemetry(ctx context.Context) error {
	shutdown, err := telemetry.InitTracing(ctx, telemetry.Config{
		ServiceName:  "identity",
		Environment:  a.cfg.Env,
		OTLPEndpoint: a.cfg.OTLPEndpoint,
	})
	if err != nil {
		return err
	}
	shutdownCtx := context.WithoutCancel(ctx)
	a.addCloser(func() error {
		return shutdown(shutdownCtx)
	})
	return nil
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

func (a *App) initContainer() {
	a.di = newContainer(a.cfg, a.log, a.pool)
}

func (a *App) initHTTPServer() {
	mux := http.NewServeMux()
	mux.Handle("GET /health/live", health.LiveHandler())
	mux.Handle("GET /health/ready", health.ReadyHandler(func(ctx context.Context) error {
		return a.pool.Ping(ctx)
	}))
	mux.Handle("GET /metrics", metrics.Handler())
	a.di.authHandler.Register(mux)

	handler := middleware.Compose(mux,
		middleware.Recovery(a.log),
		middleware.RequestID,
		middleware.CorrelationID,
		telemetry.HTTPMiddleware("identity"),
		metrics.HTTPMiddleware("identity"),
		middleware.Logging(a.log),
		a.cors(),
	)

	a.srv = &http.Server{
		Addr:         ":" + a.cfg.AppPort,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (a *App) cors() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == a.cfg.WebOrigin {
				w.Header().Set(corsHeaderAllowOrigin, origin)
				w.Header().Set(corsHeaderAllowCredentials, corsAllowCredentialsValue)
				w.Header().Set(corsHeaderVary, corsVaryOriginValue)
				w.Header().Set(corsHeaderAllowHeaders, corsAllowHeadersValue)
				w.Header().Set(corsHeaderAllowMethods, corsAllowMethodsValue)
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (a *App) addCloser(fn func() error) {
	a.closers = append(a.closers, fn)
}
