package config

import (
	"context"
	"testing"
	"time"
)

func TestLoadHTTPConfigurationAndLazyDatabase(t *testing.T) {
	t.Setenv("MCP_LOGS_TRANSPORT", "http")
	t.Setenv("MCP_LOGS_POSTGRES_DSN", "postgres://reader:unused@127.0.0.1:1/missing")
	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddress != "127.0.0.1:8090" || len(cfg.HTTPHosts) != 2 || len(cfg.HTTPOrigins) != 2 {
		t.Fatalf("HTTP defaults not loaded: %#v", cfg)
	}
	if cfg.PostgresDSN == "" {
		t.Fatal("database alias configuration missing")
	}
}

func TestLoadRejectsInvalidTransportSettings(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"transport", "MCP_LOGS_TRANSPORT", "udp"},
		{"address", "MCP_LOGS_HTTP_ADDRESS", " "},
		{"shutdown", "MCP_LOGS_SHUTDOWN_TIMEOUT", "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MCP_LOGS_TRANSPORT", "http")
			t.Setenv(tc.key, tc.value)
			if _, err := Load(context.Background()); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestDomainsParsesTheGrouping(t *testing.T) {
	cfg := &Config{DomainSpec: "core:shortener, identity ;data:analytics"}

	domains := cfg.Domains()

	if len(domains) != 2 {
		t.Fatalf("want 2 domains, got %d: %v", len(domains), domains)
	}
	if len(domains["core"]) != 2 || domains["core"][1] != "identity" {
		t.Fatalf("members must be trimmed and kept in order, got %v", domains["core"])
	}
}

func TestDomainsIgnoresMalformedGroups(t *testing.T) {
	cfg := &Config{DomainSpec: "core:shortener;;garbage;empty:"}

	domains := cfg.Domains()

	if len(domains) != 1 {
		t.Fatalf("a group without members or a separator carries no information, got %v", domains)
	}
}

func TestValidateRejectsADomainOutsideTheAllowlist(t *testing.T) {
	cfg := &Config{
		ElasticsearchURL:   "http://localhost:9200",
		ElasticsearchIndex: "logs-*",
		RequestTimeout:     time.Second,
		DiscoveryTTL:       time.Minute,
		Services:           []string{"shortener"},
		DomainSpec:         "core:shortener,identity",
	}

	// A domain naming a service the server may not reach can never be selected,
	// which reads as a broken tool rather than a configuration mistake.
	if err := cfg.Validate(); err == nil {
		t.Fatal("want an error when a domain lists a service outside the allowlist")
	}
}

func TestValidateAllowsAnEmptyAllowlist(t *testing.T) {
	cfg := &Config{
		ElasticsearchURL:   "http://localhost:9200",
		ElasticsearchIndex: "logs-*",
		RequestTimeout:     time.Second,
		DiscoveryTTL:       time.Minute,
	}

	// Empty means "everything that ships logs", which is the default now that
	// the registry is discovered rather than configured.
	if err := cfg.Validate(); err != nil {
		t.Fatalf("an empty allowlist is the normal case, got %v", err)
	}
}
