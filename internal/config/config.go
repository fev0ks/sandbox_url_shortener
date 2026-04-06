// Package config provides environment-driven configuration with go-envconfig.
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all runtime configuration for the application.
type Config struct {
	AppPort         string `env:"APP_PORT,default=8080"`
	WorkerAdminPort string `env:"WORKER_ADMIN_PORT,default=9090"`
	LogLevel        string `env:"LOG_LEVEL,default=INFO"`
	Env             string `env:"APP_ENV,default=local"`
	DBDSN           string `env:"DB_DSN,required"`
	DBMaxConns      int32  `env:"DB_MAX_CONNS,default=10"`
	BaseURL         string `env:"BASE_URL,default=http://localhost:8080"`
}

// Load reads configuration from environment variables via go-envconfig and validates.
func Load(ctx context.Context) (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process(ctx, cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate checks cross-field semantic rules that go beyond struct tags.
func (c *Config) Validate() error {
	c.LogLevel = strings.ToUpper(c.LogLevel)
	switch c.LogLevel {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("LOG_LEVEL %q is not valid; expected DEBUG, INFO, WARN, or ERROR", c.LogLevel)
	}
	return nil
}
