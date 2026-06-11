package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr     string
	DatabaseURL    string
	AdminToken     string
	CatalogPath    string
	DefaultTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:     envOrDefault("LISTEN_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AdminToken:     os.Getenv("ADMIN_TOKEN"),
		CatalogPath:    os.Getenv("CATALOG_PATH"),
		DefaultTimeout: 60 * time.Second,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN is required")
	}

	if raw := os.Getenv("REQUEST_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, fmt.Errorf("invalid REQUEST_TIMEOUT_SECONDS: %q", raw)
		}
		cfg.DefaultTimeout = time.Duration(seconds) * time.Second
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
