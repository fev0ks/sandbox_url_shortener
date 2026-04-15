// Package config provides environment-driven configuration for the analytics service.
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/sethvargo/go-envconfig"

	platformclickhouse "sandbox_url_shortener/internal/platform/clickhouse"
	"sandbox_url_shortener/internal/platform/eventing"
)

// Config holds runtime configuration for the analytics service.
type Config struct {
	AdminPort string `env:"ANALYTICS_ADMIN_PORT,default=9090"`
	LogLevel  string `env:"LOG_LEVEL,default=INFO"`
	Env       string `env:"APP_ENV,default=local"`

	KafkaBrokers             string `env:"KAFKA_BROKERS"`
	KafkaTopicLinkVisited    string `env:"KAFKA_TOPIC_LINK_VISITED"`
	KafkaTopicLinkVisitedDLQ string `env:"KAFKA_TOPIC_LINK_VISITED_DLQ"`
	KafkaConsumerGroup       string `env:"KAFKA_CONSUMER_GROUP"`
	KafkaConsumerEnabled     bool   `env:"KAFKA_CONSUMER_ENABLED,default=false"`
	OTLPEndpoint             string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`

	ClickHouseAddr     string `env:"CLICKHOUSE_ADDR"`
	ClickHouseDatabase string `env:"CLICKHOUSE_DATABASE"`
	ClickHouseUsername string `env:"CLICKHOUSE_USERNAME"`
	ClickHousePassword string `env:"CLICKHOUSE_PASSWORD"`
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
	if c.KafkaConsumerGroup == "" {
		c.KafkaConsumerGroup = eventing.DefaultAnalyticsConsumerGroup
	}
	if c.ClickHouseAddr == "" {
		c.ClickHouseAddr = platformclickhouse.DefaultAddr
	}
	if c.ClickHouseDatabase == "" {
		c.ClickHouseDatabase = platformclickhouse.DefaultDatabase
	}
	if c.ClickHouseUsername == "" {
		c.ClickHouseUsername = platformclickhouse.DefaultUsername
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
	if strings.TrimSpace(c.KafkaConsumerGroup) == "" {
		return fmt.Errorf("KAFKA_CONSUMER_GROUP must not be empty")
	}
	if strings.TrimSpace(c.ClickHouseAddr) == "" {
		return fmt.Errorf("CLICKHOUSE_ADDR must not be empty")
	}
	if strings.TrimSpace(c.ClickHouseDatabase) == "" {
		return fmt.Errorf("CLICKHOUSE_DATABASE must not be empty")
	}
	if strings.TrimSpace(c.ClickHouseUsername) == "" {
		return fmt.Errorf("CLICKHOUSE_USERNAME must not be empty")
	}
	return nil
}
