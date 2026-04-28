package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/internal/platform/logger"
	"sandbox_url_shortener/services/identity/internal/config"
	"sandbox_url_shortener/services/identity/internal/repository/postgres"
	"sandbox_url_shortener/services/identity/internal/service"
)

func main() {
	os.Exit(run())
}

func run() int {
	bootstrapLog := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		slog.String("service", "identity-maintenance"),
	)
	ctx := context.Background()

	cfg, err := config.Load(ctx)
	if err != nil {
		bootstrapLog.ErrorContext(ctx, "failed to load identity maintenance config", "error", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.MaintenanceTimeout())
	defer cancel()
	log := logger.New(cfg.LogLevel, "identity-maintenance", cfg.Env)

	if len(os.Args) != 2 || os.Args[1] != "cleanup" {
		log.ErrorContext(ctx, "unsupported identity maintenance command",
			slog.String("operation", "identity_maintenance"),
			slog.String("expected_command", "cleanup"),
		)
		return 2
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DBDSN)
	if err != nil {
		log.ErrorContext(ctx, "failed to parse identity DB_DSN", "operation", "identity_maintenance", "error", err)
		return 1
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		log.ErrorContext(ctx, "failed to create identity db pool", "operation", "identity_maintenance", "error", err)
		return 1
	}
	defer pool.Close()

	repo := postgres.NewRepository(pool)
	retention := service.NewRetentionService(
		repo,
		log,
		cfg.OAuthStateRetention(),
		cfg.SessionRetention(),
		cfg.RetentionDeleteBatchSize,
	)
	if _, err := retention.Cleanup(ctx, time.Now().UTC()); err != nil {
		log.ErrorContext(ctx, "identity maintenance cleanup failed", "operation", "identity_maintenance", "error", err)
		return 1
	}
	return 0
}
