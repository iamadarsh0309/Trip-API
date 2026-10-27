package config

import (
	"testing"
)

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("APP_PORT", "8080")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://trip:trip@localhost:5432/tripdb?sslmode=disable")
	t.Setenv("APP_PORT", "")
	t.Setenv("GIN_MODE", "")
	t.Setenv("MIGRATIONS_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppPort != "8080" || cfg.GinMode != "debug" || cfg.MigrationsPath != "file://migrations" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("APP_PORT", "abc")
	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid port error")
	}
}
