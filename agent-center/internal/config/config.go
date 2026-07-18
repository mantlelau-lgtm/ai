package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddr          string
	DatabaseURL         string
	AdminToken          string
	DefaultTimeout      time.Duration
	AgentOfflineTimeout time.Duration
}

func Load() (Config, error) {
	offlineTimeout, err := envDurationOrDefault("AGENT_OFFLINE_TIMEOUT", 90*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		ListenAddr:          envOrDefault("LISTEN_ADDR", ":9999"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		AdminToken:          os.Getenv("ADMIN_TOKEN"),
		DefaultTimeout:      15 * time.Second,
		AgentOfflineTimeout: offlineTimeout,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN is required")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDurationOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return duration, nil
}
