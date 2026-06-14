package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr                 string
	DatabaseURL                string
	AdminToken                 string
	CatalogPath                string
	AdminConfigBaseURL         string
	AdminCatalogPath           string
	AdminConfigReloadInterval  time.Duration
	DefaultTimeout             time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:                envOrDefault("LISTEN_ADDR", ":8080"),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		AdminToken:                os.Getenv("ADMIN_TOKEN"),
		CatalogPath:               os.Getenv("CATALOG_PATH"),
		AdminConfigBaseURL:        strings.TrimRight(os.Getenv("ADMIN_CONFIG_BASE_URL"), "/"),
		AdminCatalogPath:          envOrDefault("ADMIN_LLM_CATALOG_PATH", "/api/runtime/llm-gateway/catalog"),
		AdminConfigReloadInterval: 30 * time.Second,
		DefaultTimeout:            60 * time.Second,
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

	if raw := os.Getenv("ADMIN_CONFIG_RELOAD_INTERVAL"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 0 {
			return Config{}, fmt.Errorf("invalid ADMIN_CONFIG_RELOAD_INTERVAL: %q", raw)
		}
		cfg.AdminConfigReloadInterval = value
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
