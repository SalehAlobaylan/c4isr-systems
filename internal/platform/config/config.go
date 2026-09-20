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
	Version        string
	// Environment controls safety-sensitive configuration defaults. It is
	// intentionally explicit so production cannot accidentally inherit local
	// development credentials.
	Environment string
	// AuthRequired controls whether API requests need a bearer token. It is
	// enabled by Load; in-process harnesses must still declare an environment
	// explicitly so a zero-value config cannot silently bypass the safety policy.
	AuthRequired bool
	// AuthTokens maps a bearer token to an operator id. Tokens are intentionally
	// configuration-only for the first RBAC milestone; an external IdP can be
	// introduced later without changing domain services.
	AuthTokens map[string]string
}

// Load reads configuration from the process environment and applies
// documented, production-safe defaults.
func Load() (Config, error) {
	environment := strings.ToLower(strings.TrimSpace(envOr("C4ISR_ENV", EnvironmentProduction)))
	defaultLogFormat := "json"
	if environment == EnvironmentDevelopment || environment == EnvironmentTest {
		defaultLogFormat = "text"
	}
	cfg := Config{
		HTTPAddr:     envOr("C4ISR_HTTP_ADDR", ":8080"),
		DatabaseURL:  "",
		LogLevel:     envOr("C4ISR_LOG_LEVEL", "info"),
		LogFormat:    envOr("C4ISR_LOG_FORMAT", defaultLogFormat),
		ScenariosDir: envOr("C4ISR_SCENARIOS_DIR", "./scenarios"),
		Environment:  environment,
		Version:      envOr("C4ISR_VERSION", "dev"),
		AuthRequired: true,
	}
	var err error
	if cfg.DatabaseURL, err = secretEnv("C4ISR_DATABASE_URL"); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("C4ISR_DATABASE_URL or C4ISR_DATABASE_URL_FILE is required")
	}
	if raw := os.Getenv("C4ISR_ALLOWED_ORIGINS"); raw != "" {
		for _, origin := range strings.Split(raw, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, origin)
			}
		}
	}

	if raw, ok := os.LookupEnv("C4ISR_AUTH_REQUIRED"); ok {
		parsed, err := parseBool("C4ISR_AUTH_REQUIRED", raw)
		if err != nil {
			return Config{}, err
		}
		cfg.AuthRequired = parsed
	}
	if raw, err := secretEnv("C4ISR_AUTH_TOKENS"); err != nil {
		return Config{}, err
	} else if raw != "" {
		parsed, parseErr := parseAuthTokens(raw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		cfg.AuthTokens = parsed
	} else if cfg.Environment == EnvironmentDevelopment {
		// The default token is deliberately obvious and local-development only.
		// Deployments must replace it with a secret token list.
		cfg.AuthTokens = map[string]string{"dev-operator-token": "operator-01"}
	} else if cfg.AuthRequired && cfg.Environment != EnvironmentTest {
		return Config{}, fmt.Errorf("C4ISR_AUTH_TOKENS is required when authentication is enabled outside development")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

const (
	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentStaging     = "staging"
	EnvironmentProduction  = "production"
)

func validateEnvironment(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EnvironmentDevelopment, EnvironmentTest, EnvironmentStaging, EnvironmentProduction:
		return nil
	default:
		return fmt.Errorf("C4ISR_ENV must be development, test, staging, or production")
	}
}

// Validate enforces configuration safety invariants for both environment-
// loaded configuration and in-process test/application harnesses.
func (cfg Config) Validate() error {
	environment := strings.ToLower(strings.TrimSpace(cfg.Environment))
	if err := validateEnvironment(environment); err != nil {
		return err
	}
	for token, operatorID := range cfg.AuthTokens {
		if strings.TrimSpace(token) == "" || strings.TrimSpace(operatorID) == "" {
			return fmt.Errorf("C4ISR_AUTH_TOKENS entries require an operator id and token")
		}
	}
	if !cfg.AuthRequired && environment != EnvironmentDevelopment && environment != EnvironmentTest {
		return fmt.Errorf("C4ISR_AUTH_REQUIRED=false is only permitted in development or test")
	}
	if cfg.AuthRequired && len(cfg.AuthTokens) == 0 && environment != EnvironmentDevelopment && environment != EnvironmentTest {
		return fmt.Errorf("C4ISR_AUTH_TOKENS is required when authentication is enabled outside development and test")
	}
	if environment != EnvironmentDevelopment && environment != EnvironmentTest && !strings.EqualFold(strings.TrimSpace(cfg.LogFormat), "json") {
		return fmt.Errorf("C4ISR_LOG_FORMAT=json is required in staging and production")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// secretEnv reads a value from KEY or KEY_FILE. File-backed configuration is
// the deployment default so credentials can be mounted by a secret manager
// without appearing in process arguments or Compose files.
func secretEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	file := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if value != "" && file != "" {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", key, key)
	}
	if file == "" {
		return value, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", key, err)
	}
	value = strings.TrimSpace(string(raw))
	if value == "" {
		return "", fmt.Errorf("%s_FILE must contain a non-empty value", key)
	}
	return value, nil
}

func parseBool(key, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", key)
	}
}

// parseAuthTokens accepts comma-separated operator-id=token pairs. The
// returned shape is token -> operator id so request authentication can compare
// candidate secrets without exposing token values to the domain model.
func parseAuthTokens(raw string) (map[string]string, error) {
	parsed := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("C4ISR_AUTH_TOKENS entry %q must be operator-id=token", item)
		}
		operatorID := strings.TrimSpace(parts[0])
		token := strings.TrimSpace(parts[1])
		if operatorID == "" || token == "" {
			return nil, fmt.Errorf("C4ISR_AUTH_TOKENS entries require an operator id and token")
		}
		if _, exists := parsed[token]; exists {
			return nil, fmt.Errorf("C4ISR_AUTH_TOKENS contains a duplicate token")
		}
		parsed[token] = operatorID
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("C4ISR_AUTH_TOKENS must contain at least one operator-id=token entry")
	}
	return parsed, nil
}
