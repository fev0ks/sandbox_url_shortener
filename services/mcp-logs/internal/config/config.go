// Package config provides environment-driven configuration for the mcp-logs service.
package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds runtime configuration for the mcp-logs service.
type Config struct {
	LogLevel string `env:"LOG_LEVEL,default=INFO"`
	Env      string `env:"APP_ENV,default=local"`

	Transport       string        `env:"MCP_LOGS_TRANSPORT,default=stdio"`
	HTTPAddress     string        `env:"MCP_LOGS_HTTP_ADDRESS,default=127.0.0.1:8090"`
	HTTPHosts       []string      `env:"MCP_LOGS_HTTP_HOSTS,default=localhost:8090,127.0.0.1:8090"`
	HTTPOrigins     []string      `env:"MCP_LOGS_HTTP_ORIGINS,default=http://localhost:8090,http://127.0.0.1:8090"`
	ShutdownTimeout time.Duration `env:"MCP_LOGS_SHUTDOWN_TIMEOUT,default=10s"`
	PostgresDSN     string        `env:"MCP_LOGS_POSTGRES_DSN"`

	ElasticsearchURL   string        `env:"MCP_LOGS_ELASTICSEARCH_URL,default=http://localhost:9200"`
	ElasticsearchIndex string        `env:"MCP_LOGS_ELASTICSEARCH_INDEX,default=shortener-logs-*"`
	TempoURL           string        `env:"MCP_LOGS_TEMPO_URL,default=http://localhost:3200"`
	KibanaURL          string        `env:"MCP_LOGS_KIBANA_URL,default=http://localhost:5601"`
	RequestTimeout     time.Duration `env:"MCP_LOGS_REQUEST_TIMEOUT,default=15s"`

	// Upstream credentials. Empty means an unauthenticated endpoint, which is
	// what the local stack runs. Any deployment where the log index is not
	// already private needs these, and the account behind them should be
	// read-only: nothing here writes.
	ElasticsearchUsername string `env:"MCP_LOGS_ELASTICSEARCH_USERNAME"`
	ElasticsearchPassword string `env:"MCP_LOGS_ELASTICSEARCH_PASSWORD"`
	ElasticsearchAPIKey   string `env:"MCP_LOGS_ELASTICSEARCH_API_KEY"`
	TempoToken            string `env:"MCP_LOGS_TEMPO_TOKEN"`

	// Services scopes the server to a subset of what the index holds. Empty -
	// the default - means every service that actually ships logs is reachable.
	//
	// This used to be the vocabulary itself, which had two problems at scale:
	// the list was inlined into the tool schema and re-sent on every turn, and
	// a name listed but never shipped reported zero errors forever, which an
	// agent reads as a healthy service rather than an unobserved one. The
	// vocabulary is now discovered; this is an operator boundary on top of it,
	// and the fallback when discovery fails.
	Services []string `env:"MCP_LOGS_SERVICES"`

	// Domains groups services for the first narrowing step, as
	// "core:shortener,identity;data:analytics". The log schema has no domain
	// field, so the grouping is configured rather than discovered. Domains are
	// few and change rarely, which is what still makes them safe as an enum.
	DomainSpec string `env:"MCP_LOGS_DOMAINS"`

	// DiscoveryTTL is how long a discovered service registry or field list
	// stays usable. Discovery is an aggregation across the cluster and the
	// answers change on the timescale of a deployment, not a conversation.
	DiscoveryTTL time.Duration `env:"MCP_LOGS_DISCOVERY_TTL,default=5m"`
}

// Domains parses the configured domain grouping.
func (c *Config) Domains() map[string][]string {
	out := make(map[string][]string)
	for _, group := range strings.Split(c.DomainSpec, ";") {
		name, members, ok := strings.Cut(group, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		for _, member := range strings.Split(members, ",") {
			if member = strings.TrimSpace(member); member != "" {
				out[name] = append(out[name], member)
			}
		}
		if len(out[name]) == 0 {
			delete(out, name)
		}
	}
	return out
}

// Load reads configuration from environment variables and validates it.
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
	c.ElasticsearchURL = strings.TrimSuffix(strings.TrimSpace(c.ElasticsearchURL), "/")
	c.TempoURL = strings.TrimSuffix(strings.TrimSpace(c.TempoURL), "/")
	c.KibanaURL = strings.TrimSuffix(strings.TrimSpace(c.KibanaURL), "/")

	cleaned := make([]string, 0, len(c.Services))
	for _, s := range c.Services {
		if s = strings.TrimSpace(s); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	c.Services = cleaned
}

// Validate reports whether the configuration is usable.
func (c *Config) Validate() error {
	if c.Transport != "" && c.Transport != "stdio" && c.Transport != "http" {
		return fmt.Errorf("MCP_LOGS_TRANSPORT must be stdio or http")
	}
	if c.Transport == "http" {
		if strings.TrimSpace(c.HTTPAddress) == "" || len(c.HTTPHosts) == 0 {
			return fmt.Errorf("HTTP address and allowed hosts are required")
		}
		if c.ShutdownTimeout <= 0 {
			return fmt.Errorf("MCP_LOGS_SHUTDOWN_TIMEOUT must be positive")
		}
	}
	if c.ElasticsearchURL == "" {
		return fmt.Errorf("MCP_LOGS_ELASTICSEARCH_URL is required")
	}
	if c.ElasticsearchIndex == "" {
		return fmt.Errorf("MCP_LOGS_ELASTICSEARCH_INDEX is required")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("MCP_LOGS_REQUEST_TIMEOUT must be positive")
	}
	if c.DiscoveryTTL <= 0 {
		return fmt.Errorf("MCP_LOGS_DISCOVERY_TTL must be positive")
	}
	// A domain naming a service outside the allowlist can never be selected,
	// which looks like a broken tool rather than a configuration mistake.
	if len(c.Services) > 0 {
		allowed := make(map[string]bool, len(c.Services))
		for _, name := range c.Services {
			allowed[name] = true
		}
		for domain, members := range c.Domains() {
			for _, member := range members {
				if !allowed[member] {
					return fmt.Errorf("MCP_LOGS_DOMAINS: domain %q lists %q, which is not in MCP_LOGS_SERVICES", domain, member)
				}
			}
		}
	}
	return nil
}
