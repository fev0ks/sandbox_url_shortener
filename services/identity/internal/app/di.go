package app

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/services/identity/internal/config"
	"sandbox_url_shortener/services/identity/internal/repository/postgres"
	"sandbox_url_shortener/services/identity/internal/service"
	authhandler "sandbox_url_shortener/services/identity/internal/transport/http/stdlib/auth"
)

type container struct {
	repo        *postgres.Repository
	authSvc     *service.AuthService
	oauthSvc    *service.OAuthService
	authHandler *authhandler.Handler
}

func newContainer(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) *container {
	c := &container{}
	c.repo = postgres.NewRepository(pool)
	c.authSvc = service.NewAuthService(c.repo, log, cfg.SessionAbsoluteLifetime(), cfg.SessionIdleTimeout())
	c.oauthSvc = service.NewOAuthService(
		c.repo,
		c.authSvc,
		service.NewGoogleProvider(cfg),
		service.NewAppleProvider(cfg),
	)
	c.authHandler = authhandler.NewHandler(
		c.authSvc,
		c.oauthSvc,
		log,
		cfg.WebAuthCallbackURL,
		cfg.CookieSecure,
		cfg.CookieDomain,
		cfg.SessionAbsoluteLifetime(),
	)
	return c
}
