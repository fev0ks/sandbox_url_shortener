package config

import (
	"context"
	"testing"
)

func TestConfigValidateDefaults(t *testing.T) {
	cfg := &Config{
		AppPort:                   "8081",
		LogLevel:                  "info",
		DBDSN:                     "postgres://user:pass@localhost:5432/db?sslmode=disable",
		PublicBaseURL:             "http://localhost:8081",
		WebOrigin:                 "http://localhost:3000",
		WebAuthCallbackURL:        "http://localhost:3000/auth/callback",
		SessionAbsoluteDays:       30,
		SessionIdleDays:           7,
		OAuthStateRetentionHours:  24,
		SessionRetentionDays:      30,
		RetentionDeleteBatchSize:  1000,
		MaintenanceTimeoutSeconds: 120,
	}
	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if cfg.GoogleRedirectURL != "http://localhost:8081/api/v1/auth/oauth/google/callback" {
		t.Fatalf("GoogleRedirectURL = %q", cfg.GoogleRedirectURL)
	}
	if cfg.AppleRedirectURL != "http://localhost:8081/api/v1/auth/oauth/apple/callback" {
		t.Fatalf("AppleRedirectURL = %q", cfg.AppleRedirectURL)
	}
	if cfg.OAuthStateRetention().Hours() != 24 {
		t.Fatalf("OAuthStateRetention = %s", cfg.OAuthStateRetention())
	}
	if cfg.SessionRetention().Hours() != 30*24 {
		t.Fatalf("SessionRetention = %s", cfg.SessionRetention())
	}
	if cfg.MaintenanceTimeout().Seconds() != 120 {
		t.Fatalf("MaintenanceTimeout = %s", cfg.MaintenanceTimeout())
	}
}

func TestConfigValidateRejectsBadSessionWindow(t *testing.T) {
	cfg := &Config{
		LogLevel:                  "INFO",
		DBDSN:                     "postgres://user:pass@localhost:5432/db?sslmode=disable",
		PublicBaseURL:             "http://localhost:8081",
		WebOrigin:                 "http://localhost:3000",
		WebAuthCallbackURL:        "http://localhost:3000/auth/callback",
		SessionAbsoluteDays:       7,
		SessionIdleDays:           30,
		OAuthStateRetentionHours:  24,
		SessionRetentionDays:      30,
		RetentionDeleteBatchSize:  1000,
		MaintenanceTimeoutSeconds: 120,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConfigValidateRejectsBadRetentionWindow(t *testing.T) {
	cfg := &Config{
		LogLevel:                  "INFO",
		DBDSN:                     "postgres://user:pass@localhost:5432/db?sslmode=disable",
		PublicBaseURL:             "http://localhost:8081",
		WebOrigin:                 "http://localhost:3000",
		WebAuthCallbackURL:        "http://localhost:3000/auth/callback",
		SessionAbsoluteDays:       30,
		SessionIdleDays:           7,
		OAuthStateRetentionHours:  0,
		SessionRetentionDays:      30,
		RetentionDeleteBatchSize:  1000,
		MaintenanceTimeoutSeconds: 120,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.OAuthStateRetentionHours = 24
	cfg.SessionRetentionDays = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.SessionRetentionDays = 30
	cfg.RetentionDeleteBatchSize = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.RetentionDeleteBatchSize = 1000
	cfg.MaintenanceTimeoutSeconds = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestConfigValidateRejectsAppleOAuthWithoutSecureCookies(t *testing.T) {
	cfg := &Config{
		LogLevel:                  "INFO",
		DBDSN:                     "postgres://user:pass@localhost:5432/db?sslmode=disable",
		PublicBaseURL:             "http://localhost:8081",
		WebOrigin:                 "http://localhost:3000",
		WebAuthCallbackURL:        "http://localhost:3000/auth/callback",
		CookieSecure:              false,
		SessionAbsoluteDays:       30,
		SessionIdleDays:           7,
		OAuthStateRetentionHours:  24,
		SessionRetentionDays:      30,
		RetentionDeleteBatchSize:  1000,
		MaintenanceTimeoutSeconds: 120,
		AppleClientID:             "com.example.web",
		AppleTeamID:               "team",
		AppleKeyID:                "key",
		ApplePrivateKeyPEM:        "pem",
		AppleRedirectURL:          "http://localhost:8081/api/v1/auth/oauth/apple/callback",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadRequiresDBDSN(t *testing.T) {
	t.Setenv("DB_DSN", "")
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected required DB_DSN error")
	}
}
