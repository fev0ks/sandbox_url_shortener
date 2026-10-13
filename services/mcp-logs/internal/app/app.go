// Package app wires the mcp-logs service together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/services/mcp-logs/internal/config"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/elasticsearch"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/postgresql"
	"sandbox_url_shortener/services/mcp-logs/internal/repository/tempo"
	"sandbox_url_shortener/services/mcp-logs/internal/service"
	"sandbox_url_shortener/services/mcp-logs/internal/transport/mcp"
)

const serviceName = "mcp-logs"

// App holds the wired dependencies of the service.
type App struct {
	cfg         *config.Config
	log         *slog.Logger
	server      *mcp.Server
	httpHandler *mcp.HTTPHandler
	database    *postgresql.Repo
}

// NewApp loads configuration and builds the dependency graph.
func NewApp(ctx context.Context) (*App, error) {
	cfg, err := config.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// stdout carries the MCP protocol stream, so logs must go to stderr.
	log := logger.NewWithWriter(os.Stderr, cfg.LogLevel, serviceName, cfg.Env)

	client := &http.Client{Timeout: cfg.RequestTimeout}
	logsRepo := elasticsearch.NewRepo(client, cfg.ElasticsearchURL, cfg.ElasticsearchIndex, elasticsearch.Auth{
		Username: cfg.ElasticsearchUsername,
		Password: cfg.ElasticsearchPassword,
		APIKey:   cfg.ElasticsearchAPIKey,
	})
	traceRepo := tempo.NewRepo(client, cfg.TempoURL, tempo.Auth{Token: cfg.TempoToken})

	investigation := service.NewInvestigation(logsRepo, traceRepo, service.Options{
		Allowlist:    cfg.Services,
		Domains:      cfg.Domains(),
		DiscoveryTTL: cfg.DiscoveryTTL,
		KibanaURL:    cfg.KibanaURL,
	})

	repos := make(map[string]service.DatabaseRepository)
	var database *postgresql.Repo
	if cfg.PostgresDSN != "" {
		database, err = postgresql.NewRepo(ctx, cfg.PostgresDSN)
		if err != nil {
			return nil, fmt.Errorf("configure PostgreSQL investigation: %w", err)
		}
		repos["shortener"] = database
	}
	databases := service.NewDatabaseInvestigation(repos, service.DatabaseOptions{})
	core := mcp.NewServer(investigation, log, databases)
	var handler *mcp.HTTPHandler
	if cfg.Transport == "http" {
		handler, err = mcp.NewHTTPHandler(core, developmentKeys(), mcp.HTTPOptions{
			Hosts: cfg.HTTPHosts, Origins: cfg.HTTPOrigins, RequestTimeout: cfg.RequestTimeout,
		}, log)
		if err != nil {
			if database != nil {
				database.Close()
			}
			return nil, fmt.Errorf("configure HTTP MCP: %w", err)
		}
	}
	return &App{
		cfg:         cfg,
		log:         log,
		server:      core,
		httpHandler: handler,
		database:    database,
	}, nil
}

// Run serves the selected transport until shutdown without requiring healthy upstreams.
func (a *App) Run(ctx context.Context) error {
	if a.database != nil {
		defer a.database.Close()
	}
	a.log.InfoContext(ctx, "mcp-logs started",
		slog.String("elasticsearch", a.cfg.ElasticsearchURL),
		slog.String("index", a.cfg.ElasticsearchIndex),
		slog.String("tempo", a.cfg.TempoURL),
		slog.Any("service_allowlist", a.cfg.Services),
		slog.Any("domains", a.cfg.Domains()),
		slog.Duration("discovery_ttl", a.cfg.DiscoveryTTL),
		slog.String("transport", a.cfg.Transport),
	)

	var err error
	if a.httpHandler != nil {
		err = a.runHTTP(ctx)
	} else {
		err = a.server.Serve(ctx, os.Stdin, os.Stdout)
	}
	if err != nil {
		return fmt.Errorf("serve mcp: %w", err)
	}

	a.log.InfoContext(ctx, "mcp-logs stopped")
	return nil
}

func (a *App) runHTTP(ctx context.Context) error {
	server := &http.Server{
		Addr: a.cfg.HTTPAddress, Handler: a.httpHandler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: time.Minute,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen MCP HTTP: %w", err)
	}
	a.log.InfoContext(ctx, "MCP HTTP listening", slog.String("address", listener.Addr().String()))
	serveError := make(chan error, 1)
	go func() { serveError <- server.Serve(listener) }()
	select {
	case err = <-serveError:
	case <-ctx.Done():
	}
	// Closing sessions cancels pending tools and GET streams before HTTP shutdown waits.
	closeErr := a.httpHandler.Close()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, closeErr, shutdownErr)
}
