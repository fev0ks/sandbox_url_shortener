// Package config provides environment-driven configuration for the shortener service.
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/sethvargo/go-envconfig"

	"sandbox_url_shortener/internal/platform/eventing"
)

// Config holds runtime configuration for the shortener service.
type Config struct {
	AppPort    string `env:"APP_PORT,default=8080"`
	LogLevel   string `env:"LOG_LEVEL,default=INFO"`
	Env        string `env:"APP_ENV,default=local"`
	DBDSN      string `env:"DB_DSN,required"`
	DBMaxConns int32  `env:"DB_MAX_CONNS,default=10"`
	BaseURL    string `env:"BASE_URL,default=http://localhost:8080"`
	RedisAddr  string `env:"REDIS_ADDR,default=localhost:6379"`
	RedisDB    int    `env:"REDIS_DB,default=0"`

	RateLimitPerMin int `env:"RATE_LIMIT_PER_MIN,default=10"`

	KafkaBrokers             string `env:"KAFKA_BROKERS"`
	KafkaTopicLinkVisited    string `env:"KAFKA_TOPIC_LINK_VISITED"`
	KafkaTopicLinkVisitedDLQ string `env:"KAFKA_TOPIC_LINK_VISITED_DLQ"`
	KafkaProducerEnabled     bool   `env:"KAFKA_PRODUCER_ENABLED,default=false"`
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
	if c.KafkaBrokers == "" {
		c.KafkaBrokers = eventing.DefaultKafkaBrokers
	}
	if c.KafkaTopicLinkVisited == "" {
		c.KafkaTopicLinkVisited = eventing.DefaultLinkVisitedTopic
	}
	if c.KafkaTopicLinkVisitedDLQ == "" {
		c.KafkaTopicLinkVisitedDLQ = eventing.DefaultLinkVisitedDLQTopic
	}
}

// Validate checks cross-field semantic rules that go beyond struct tags.
func (c *Config) Validate() error {
	c.LogLevel = strings.ToUpper(c.LogLevel)
	switch c.LogLevel {
	case "DEBUG", "INFO", "WARN", "ERROR":
	default:
		return fmt.Errorf("LOG_LEVEL %q is not valid; expected DEBUG, INFO, WARN, or ERROR", c.LogLevel)
	}
	if strings.TrimSpace(c.KafkaBrokers) == "" {
		return fmt.Errorf("KAFKA_BROKERS must not be empty")
	}
	if strings.TrimSpace(c.KafkaTopicLinkVisited) == "" {
		return fmt.Errorf("KAFKA_TOPIC_LINK_VISITED must not be empty")
	}
	if strings.TrimSpace(c.KafkaTopicLinkVisitedDLQ) == "" {
		return fmt.Errorf("KAFKA_TOPIC_LINK_VISITED_DLQ must not be empty")
	}
	return nil
}
