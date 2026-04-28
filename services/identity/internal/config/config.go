// Package config provides environment-driven configuration for the identity service.
package config

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds runtime configuration for the identity service.
type Config struct {
	AppPort    string `env:"IDENTITY_APP_PORT,default=8081"`
	LogLevel   string `env:"LOG_LEVEL,default=INFO"`
	Env        string `env:"APP_ENV,default=local"`
	DBDSN      string `env:"DB_DSN,required"`
	DBMaxConns int32  `env:"DB_MAX_CONNS,default=10"`

	PublicBaseURL             string `env:"IDENTITY_PUBLIC_BASE_URL,default=http://localhost:8081"`
	WebOrigin                 string `env:"WEB_ORIGIN,default=http://localhost:3000"`
	WebAuthCallbackURL        string `env:"WEB_AUTH_CALLBACK_URL,default=http://localhost:3000/auth/callback"`
	CookieSecure              bool   `env:"IDENTITY_COOKIE_SECURE,default=true"`
	CookieDomain              string `env:"IDENTITY_COOKIE_DOMAIN"`
	SessionAbsoluteDays       int    `env:"IDENTITY_SESSION_ABSOLUTE_DAYS,default=30"`
	SessionIdleDays           int    `env:"IDENTITY_SESSION_IDLE_DAYS,default=7"`
	OAuthStateRetentionHours  int    `env:"IDENTITY_OAUTH_STATE_RETENTION_HOURS,default=24"`
	SessionRetentionDays      int    `env:"IDENTITY_SESSION_RETENTION_DAYS,default=30"`
	RetentionDeleteBatchSize  int    `env:"IDENTITY_RETENTION_DELETE_BATCH_SIZE,default=1000"`
	MaintenanceTimeoutSeconds int    `env:"IDENTITY_MAINTENANCE_TIMEOUT_SECONDS,default=120"`

	GoogleClientID     string `env:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleRedirectURL  string `env:"GOOGLE_OAUTH_REDIRECT_URL"`

	AppleClientID      string `env:"APPLE_OAUTH_CLIENT_ID"`
	AppleTeamID        string `env:"APPLE_OAUTH_TEAM_ID"`
	AppleKeyID         string `env:"APPLE_OAUTH_KEY_ID"`
	ApplePrivateKeyPEM string `env:"APPLE_OAUTH_PRIVATE_KEY_PEM"`
	AppleRedirectURL   string `env:"APPLE_OAUTH_REDIRECT_URL"`

	OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
}

// Load reads configuration from environment variables via go-envconfig and validates it.
func Load(ctx context.Context) (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process(ctx, cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	c.LogLevel = strings.ToUpper(c.LogLevel)
	if c.GoogleRedirectURL == "" {
		c.GoogleRedirectURL = strings.TrimRight(c.PublicBaseURL, "/") + "/api/v1/auth/oauth/google/callback"
	}
	if c.AppleRedirectURL == "" {
		c.AppleRedirectURL = strings.TrimRight(c.PublicBaseURL, "/") + "/api/v1/auth/oauth/apple/callback"
	}
}

// Validate checks cross-field semantic rules that go beyond struct tags.
func (c *Config) Validate() error {
	switch c.LogLevel {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("LOG_LEVEL %q is not valid; expected DEBUG, INFO, WARN, or ERROR", c.LogLevel)
	}
	if strings.TrimSpace(c.DBDSN) == "" {
		return fmt.Errorf("DB_DSN must not be empty")
	}
	for name, raw := range map[string]string{
		"IDENTITY_PUBLIC_BASE_URL": c.PublicBaseURL,
		"WEB_ORIGIN":               c.WebOrigin,
		"WEB_AUTH_CALLBACK_URL":    c.WebAuthCallbackURL,
	} {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("%s must be an absolute URL", name)
		}
	}
	if c.SessionAbsoluteLifetime() <= 0 {
		return fmt.Errorf("IDENTITY_SESSION_ABSOLUTE_DAYS must be positive")
	}
	if c.SessionIdleTimeout() <= 0 {
		return fmt.Errorf("IDENTITY_SESSION_IDLE_DAYS must be positive")
	}
	if c.SessionIdleTimeout() > c.SessionAbsoluteLifetime() {
		return fmt.Errorf("IDENTITY_SESSION_IDLE_DAYS must be <= IDENTITY_SESSION_ABSOLUTE_DAYS")
	}
	if c.OAuthStateRetention() <= 0 {
		return fmt.Errorf("IDENTITY_OAUTH_STATE_RETENTION_HOURS must be positive")
	}
	if c.SessionRetention() <= 0 {
		return fmt.Errorf("IDENTITY_SESSION_RETENTION_DAYS must be positive")
	}
	if c.RetentionDeleteBatchSize <= 0 {
		return fmt.Errorf("IDENTITY_RETENTION_DELETE_BATCH_SIZE must be positive")
	}
	if c.MaintenanceTimeout() <= 0 {
		return fmt.Errorf("IDENTITY_MAINTENANCE_TIMEOUT_SECONDS must be positive")
	}
	if c.AppleOAuthEnabled() && !c.CookieSecure {
		return fmt.Errorf("IDENTITY_COOKIE_SECURE must be true when Apple OAuth is configured")
	}
	return nil
}

// SessionAbsoluteLifetime returns the maximum session lifetime.
func (c *Config) SessionAbsoluteLifetime() time.Duration {
	return time.Duration(c.SessionAbsoluteDays) * 24 * time.Hour
}

// SessionIdleTimeout returns the idle timeout used to slide session expiry.
func (c *Config) SessionIdleTimeout() time.Duration {
	return time.Duration(c.SessionIdleDays) * 24 * time.Hour
}

// OAuthStateRetention returns how long expired or consumed OAuth state rows are retained.
func (c *Config) OAuthStateRetention() time.Duration {
	return time.Duration(c.OAuthStateRetentionHours) * time.Hour
}

// SessionRetention returns how long revoked or expired session rows are retained.
func (c *Config) SessionRetention() time.Duration {
	return time.Duration(c.SessionRetentionDays) * 24 * time.Hour
}

// MaintenanceTimeout returns the maximum duration for one maintenance command run.
func (c *Config) MaintenanceTimeout() time.Duration {
	return time.Duration(c.MaintenanceTimeoutSeconds) * time.Second
}

// GoogleOAuthEnabled reports whether Google OAuth has enough config to start.
func (c *Config) GoogleOAuthEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

// AppleOAuthEnabled reports whether Apple OAuth has enough config to start.
func (c *Config) AppleOAuthEnabled() bool {
	return c.AppleClientID != "" && c.AppleTeamID != "" && c.AppleKeyID != "" &&
		c.ApplePrivateKeyPEM != "" && c.AppleRedirectURL != ""
}
