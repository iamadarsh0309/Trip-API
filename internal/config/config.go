package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AppPort        string
	GinMode        string
	DatabaseURL    string
	MigrationsPath string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppPort:        getEnv("APP_PORT", "8080"),
		GinMode:        getEnv("GIN_MODE", "debug"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "file://migrations"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if _, err := strconv.Atoi(cfg.AppPort); err != nil {
		return nil, fmt.Errorf("APP_PORT must be a number: %w", err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
