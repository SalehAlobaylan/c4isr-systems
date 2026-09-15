// Package config loads server configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config is the runtime configuration of the C4ISR server.
type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	LogLevel       string
	LogFormat      string
	ScenariosDir   string
	AllowedOrigins []string
}

// Load reads configuration from the process environment and applies
// documented defaults suitable for local development.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:     envOr("C4ISR_HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("C4ISR_DATABASE_URL"),
		LogLevel:     envOr("C4ISR_LOG_LEVEL", "info"),
		LogFormat:    envOr("C4ISR_LOG_FORMAT", "text"),
		ScenariosDir: envOr("C4ISR_SCENARIOS_DIR", "./scenarios"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("C4ISR_DATABASE_URL is required")
	}
	if raw := os.Getenv("C4ISR_ALLOWED_ORIGINS"); raw != "" {
		for _, origin := range strings.Split(raw, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
			}
		}
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
