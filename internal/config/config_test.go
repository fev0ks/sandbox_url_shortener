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

func TestLoad_MissingDBDSN(t *testing.T) {
	mustUnsetenv(t, "DB_DSN")
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected error for missing DB_DSN, got nil")
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "TRACE")
	_, err := Load(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL, got nil")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/test")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("APP_PORT", "3000")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppPort != "3000" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "3000")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "DEBUG")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/test")
	mustUnsetenv(t, "APP_PORT")
	mustUnsetenv(t, "WORKER_ADMIN_PORT")
	mustUnsetenv(t, "LOG_LEVEL")

	cfg, err := Load(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppPort != "8080" {
		t.Errorf("AppPort = %q, want %q", cfg.AppPort, "8080")
	}
	if cfg.WorkerAdminPort != "9090" {
		t.Errorf("WorkerAdminPort = %q, want %q", cfg.WorkerAdminPort, "9090")
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
	if cfg.DBMaxConns != 10 {
		t.Errorf("DBMaxConns = %d, want %d", cfg.DBMaxConns, 10)
	}
}
