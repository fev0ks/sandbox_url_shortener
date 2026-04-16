package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	platformclickhouse "sandbox_url_shortener/internal/platform/clickhouse"
	"sandbox_url_shortener/internal/platform/health"
	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/internal/platform/middleware"
	"sandbox_url_shortener/internal/platform/telemetry"
	"sandbox_url_shortener/services/analytics/internal/config"
	clickhouserepo "sandbox_url_shortener/services/analytics/internal/repository/clickhouse"
	"sandbox_url_shortener/services/analytics/internal/service"
	statshandler "sandbox_url_shortener/services/analytics/internal/transport/http/stdlib/stats"
)

// App is the composition root for the analytics binary.
type App struct {
	cfg        *config.Config
	log        *slog.Logger
	srv        *http.Server
	clickhouse chdriver.Conn
	stats      *statshandler.Handler
	closers    []func() error
	consumers  []Consumer
}

// NewApp initializes all dependencies and returns a ready-to-run analytics App.
func NewApp(ctx context.Context) (*App, error) {
	a := &App{}

	if err := a.initConfig(ctx); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	a.initLogger()
	if err := a.initTelemetry(ctx); err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}
	if err := a.initClickHouse(ctx); err != nil {
		return nil, fmt.Errorf("init clickhouse: %w", err)
	}
	a.initStats()
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

func (a *App) initTelemetry(ctx context.Context) error {
	shutdown, err := telemetry.InitTracing(ctx, telemetry.Config{
		ServiceName:  "analytics",
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

func (a *App) initConsumers(ctx context.Context) error {
	sink := clickhouserepo.NewLinkVisitSink(a.clickhouse)
	linkVisited, err := newLinkVisitedConsumerWithSink(ctx, a.cfg, a.log, sink)
	if err != nil {
		return err
	}
	if linkVisited != nil {
		a.consumers = append(a.consumers, linkVisited)
		a.addCloser(linkVisited.Close)
	}
	return nil
}

func (a *App) initClickHouse(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := platformclickhouse.Open(
		checkCtx,
		a.cfg.ClickHouseAddr,
		a.cfg.ClickHouseDatabase,
		a.cfg.ClickHouseUsername,
		a.cfg.ClickHousePassword,
	)
	if err != nil {
		return err
	}

	a.clickhouse = conn
	a.addCloser(conn.Close)
	return nil
}

func (a *App) initStats() {
	repo := clickhouserepo.NewLinkStatsRepo(a.clickhouse)
	svc := service.NewLinkStatsService(repo, a.log)
	a.stats = statshandler.NewHandler(svc, a.log)
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
	a.stats.Register(mux)

	handler := middleware.Compose(mux,
		middleware.Recovery(a.log),
		middleware.RequestID,
		middleware.CorrelationID,
		telemetry.HTTPMiddleware("analytics"),
		metrics.HTTPMiddleware("analytics"),
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
	if a.clickhouse != nil {
		if err := a.clickhouse.Ping(ctx); err != nil {
			return fmt.Errorf("clickhouse not ready: %w", err)
		}
	}
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
