package config

import (
	"context"
	"os"
	"testing"
)

func mustUnsetenv(t *testing.T, key string) {
	t.Helper()

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "TRACE")
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL, got nil")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("ANALYTICS_ADMIN_PORT", "9191")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AdminPort != "9191" {
		t.Errorf("AdminPort = %q, want %q", cfg.AdminPort, "9191")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "DEBUG")
	}
}

func TestLoad_Defaults(t *testing.T) {
	mustUnsetenv(t, "ANALYTICS_ADMIN_PORT")
	mustUnsetenv(t, "LOG_LEVEL")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AdminPort != "9090" {
		t.Errorf("AdminPort = %q, want %q", cfg.AdminPort, "9090")
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
	if cfg.Env != "local" {
		t.Errorf("Env = %q, want %q", cfg.Env, "local")
	}
}
