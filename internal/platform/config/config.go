// Package config loads server configuration from environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SalehAlobaylan/c4isr-systems/internal/platform/freshness"
)

// Config is the runtime configuration of the C4ISR server.
type Config struct {
	ProductProfile                  string
	EnvironmentLibraryEnabled       bool
	EnvironmentToolchain            string
	EnvironmentEnrichmentToolchain  string
	EnvironmentObservationToolchain string
	EnvironmentCoastalToolchain     string
	EnvironmentDisplayToolchain     string
	EnvironmentLayerToolchain       string
	EnvironmentSpool                string
	EnvironmentParallel             int

	ArtifactS3Endpoint  string
	ArtifactS3Bucket    string
	ArtifactS3Region    string
	ArtifactS3AccessKey string
	ArtifactS3SecretKey string
	ArtifactS3AllowHTTP bool
	// PublicConnectorsFile is an optional trusted location manifest; empty means no external retrieval.
	PublicConnectorsFile string
	RuntimeBuilds        []string
	HTTPAddr             string
	DatabaseURL          string
	LogLevel             string
	LogFormat            string
	ScenariosDir         string
	AllowedOrigins       []string
	Version              string
	// TelemetryStaleAfter is the age at which an entity stops being
	// considered current. It retains its original name because it was the
	// first freshness threshold configured; it is now the middle band of
	// the freshness policy.
	TelemetryStaleAfter time.Duration
	// FreshnessDelayedAfter is the age at which an entity starts being
	// flagged as aging. FreshnessUnavailableAfter is the age at which
	// contact is treated as lost even if no link-down signal arrived.
	FreshnessDelayedAfter     time.Duration
	FreshnessUnavailableAfter time.Duration
	// Environment controls safety-sensitive configuration defaults. It is
	// intentionally explicit so production cannot accidentally inherit local
	// development credentials.
	Environment string
	// AuthRequired controls whether API requests need a bearer token. It is
	// enabled by Load; in-process harnesses must still declare an environment
	// explicitly so a zero-value config cannot silently bypass the safety policy.
	AuthRequired bool
	// DatabaseAuth permits startup with provisioned database credentials and no
	// configuration-backed bootstrap tokens. Authentication remains required.
	DatabaseAuth bool
	// AuthTokens maps bootstrap bearer tokens to legacy operator identities.
	// The identity store persists hashes, expiry, and revocation. Provisioned
	// database credentials can replace these with DatabaseAuth enabled.
	AuthTokens map[string]string
}

// FreshnessPolicy projects the configured thresholds onto the freshness
// classification the API and the interface share. The server is the only place
// these thresholds are evaluated; the browser renders the resulting state.
func (cfg Config) FreshnessPolicy() freshness.Policy {
	return freshness.Policy{
		DelayedAfter:     cfg.FreshnessDelayedAfter,
		StaleAfter:       cfg.TelemetryStaleAfter,
		UnavailableAfter: cfg.FreshnessUnavailableAfter,
	}.Normalized()
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
		ProductProfile:       envOr("C4ISR_PRODUCT_PROFILE", "map"),
		ArtifactS3Endpoint:   strings.TrimSpace(os.Getenv("C4ISR_ARTIFACT_S3_ENDPOINT")),
		ArtifactS3Bucket:     strings.TrimSpace(os.Getenv("C4ISR_ARTIFACT_S3_BUCKET")),
		ArtifactS3Region:     envOr("C4ISR_ARTIFACT_S3_REGION", "us-east-1"),
		PublicConnectorsFile: strings.TrimSpace(os.Getenv("C4ISR_PUBLIC_CONNECTORS_FILE")),
		RuntimeBuilds:        strings.Fields(os.Getenv("C4ISR_RUNTIME_BUILDS")),
		HTTPAddr:             envOr("C4ISR_HTTP_ADDR", ":8080"),
		DatabaseURL:          "",
		LogLevel:             envOr("C4ISR_LOG_LEVEL", "info"),
		LogFormat:            envOr("C4ISR_LOG_FORMAT", defaultLogFormat),
		ScenariosDir:         envOr("C4ISR_SCENARIOS_DIR", "./scenarios"),
		Environment:          environment,
		Version:              envOr("C4ISR_VERSION", "dev"),
		TelemetryStaleAfter:  5 * time.Minute,
		AuthRequired:         true,
	}
	defaults := freshness.DefaultPolicy()
	var err error
	if raw := os.Getenv("C4ISR_TELEMETRY_STALE_AFTER"); raw != "" {
		cfg.TelemetryStaleAfter, err = time.ParseDuration(strings.TrimSpace(raw))
		if err != nil || cfg.TelemetryStaleAfter <= 0 {
			return Config{}, fmt.Errorf("C4ISR_TELEMETRY_STALE_AFTER must be a positive duration")
		}
	}
	cfg.FreshnessDelayedAfter, err = durationEnv("C4ISR_FRESHNESS_DELAYED_AFTER", defaults.DelayedAfter)
	if err != nil {
		return Config{}, err
	}
	cfg.FreshnessUnavailableAfter, err = durationEnv("C4ISR_FRESHNESS_UNAVAILABLE_AFTER", defaults.UnavailableAfter)
	if err != nil {
		return Config{}, err
	}
	// Repairing the ordering here means every consumer of the config, and
	// every operator reading it, sees the same effective bands.
	policy := cfg.FreshnessPolicy()
	cfg.FreshnessDelayedAfter = policy.DelayedAfter
	cfg.TelemetryStaleAfter = policy.StaleAfter
	cfg.FreshnessUnavailableAfter = policy.UnavailableAfter
	if cfg.DatabaseURL, err = LoadDatabaseURL(); err != nil {
		return Config{}, err
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
	if raw, ok := os.LookupEnv("C4ISR_DATABASE_AUTH"); ok {
		cfg.DatabaseAuth, err = parseBool("C4ISR_DATABASE_AUTH", raw)
		if err != nil {
			return Config{}, err
		}
	}
	if raw, err := secretEnv("C4ISR_AUTH_TOKENS"); err != nil {
		return Config{}, err
	} else if raw != "" {
		parsed, parseErr := parseAuthTokens(raw)
		if parseErr != nil {
			return Config{}, parseErr
		}
		cfg.AuthTokens = parsed
	} else if cfg.Environment == EnvironmentDevelopment && !cfg.DatabaseAuth {
		// The default token is deliberately obvious and local-development only.
		// Deployments must replace it with a secret token list.
		cfg.AuthTokens = map[string]string{"dev-operator-token": "operator-01"}
	} else if cfg.AuthRequired && !cfg.DatabaseAuth && cfg.Environment != EnvironmentTest {
		return Config{}, fmt.Errorf("C4ISR_AUTH_TOKENS is required when authentication is enabled outside development")
	}
	if cfg.ArtifactS3AccessKey, err = secretEnv("C4ISR_ARTIFACT_S3_ACCESS_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.ArtifactS3SecretKey, err = secretEnv("C4ISR_ARTIFACT_S3_SECRET_KEY"); err != nil {
		return Config{}, err
	}
	if !cfg.MapOnly() {
		if value, e := secretEnv("C4ISR_RUNTIME_BUILDS"); e != nil {
			return Config{}, e
		} else if value != "" {
			cfg.RuntimeBuilds = strings.Fields(value)
		}
	} else {
		cfg.RuntimeBuilds = nil
	}
	if value := os.Getenv("C4ISR_ARTIFACT_S3_ALLOW_HTTP"); value != "" {
		cfg.ArtifactS3AllowHTTP, err = parseBool("C4ISR_ARTIFACT_S3_ALLOW_HTTP", value)
		if err != nil {
			return Config{}, err
		}
	}
	if value := os.Getenv("C4ISR_ENVIRONMENT_LIBRARY_ENABLED"); value != "" {
		cfg.EnvironmentLibraryEnabled, err = parseBool("C4ISR_ENVIRONMENT_LIBRARY_ENABLED", value)
		if err != nil {
			return Config{}, err
		}
	}
	cfg.EnvironmentToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_TOOLCHAIN"))
	cfg.EnvironmentEnrichmentToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_ENRICHMENT_TOOLCHAIN"))
	cfg.EnvironmentObservationToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_OBSERVATION_TOOLCHAIN"))
	cfg.EnvironmentCoastalToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_COASTAL_TOOLCHAIN"))
	cfg.EnvironmentDisplayToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_DISPLAY_TOOLCHAIN"))
	cfg.EnvironmentLayerToolchain = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_LAYER_TOOLCHAIN"))
	cfg.EnvironmentSpool = strings.TrimSpace(os.Getenv("C4ISR_ENVIRONMENT_SPOOL"))
	cfg.EnvironmentParallel = 1
	if environment == EnvironmentStaging {
		cfg.EnvironmentParallel = 2
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
const ProfileMap = "map"
const ProfileHistoricalFull = "historical-full"

// Profile keeps embedded legacy harnesses compatible; environment-loaded
// deployments explicitly default to the map product.
func (cfg Config) Profile() string {
	if cfg.ProductProfile == "" {
		return ProfileHistoricalFull
	}
	return cfg.ProductProfile
}
func (cfg Config) MapOnly() bool { return cfg.Profile() == ProfileMap }

func (cfg Config) Validate() error {
	if d := cfg.EnvironmentLayerToolchain; d != "" {
		raw, err := hex.DecodeString(d)
		if err != nil || len(raw) != 32 || d != strings.ToLower(d) || d == cfg.EnvironmentDisplayToolchain ||
			d == cfg.EnvironmentToolchain || d == cfg.EnvironmentEnrichmentToolchain ||
			d == cfg.EnvironmentObservationToolchain || d == cfg.EnvironmentCoastalToolchain {
			return fmt.Errorf("geographic layer generation requires its own pinned toolchain digest")
		}
	}
	if d := cfg.EnvironmentCoastalToolchain; d != "" {
		raw, err := hex.DecodeString(d)
		if err != nil || len(raw) != 32 || d != strings.ToLower(d) || d == cfg.EnvironmentToolchain || d == cfg.EnvironmentDisplayToolchain ||
			d == cfg.EnvironmentEnrichmentToolchain || d == cfg.EnvironmentObservationToolchain {
			return fmt.Errorf("coastal preparation requires its own pinned toolchain digest")
		}
	}
	if d := cfg.EnvironmentObservationToolchain; d != "" {
		raw, err := hex.DecodeString(d)
		if err != nil || len(raw) != 32 || d != strings.ToLower(d) ||
			d == cfg.EnvironmentToolchain || d == cfg.EnvironmentDisplayToolchain || d == cfg.EnvironmentEnrichmentToolchain {
			return fmt.Errorf("geographic observations require their own pinned toolchain digest")
		}
	}
	if cfg.EnvironmentEnrichmentToolchain != "" {
		raw, err := hex.DecodeString(cfg.EnvironmentEnrichmentToolchain)
		if err != nil || len(raw) != 32 || cfg.EnvironmentEnrichmentToolchain != strings.ToLower(cfg.EnvironmentEnrichmentToolchain) ||
			cfg.EnvironmentEnrichmentToolchain == cfg.EnvironmentToolchain || cfg.EnvironmentEnrichmentToolchain == cfg.EnvironmentDisplayToolchain {
			return fmt.Errorf("geographic enrichment requires its own pinned toolchain digest")
		}
	}
	if cfg.EnvironmentDisplayToolchain != "" {
		raw, err := hex.DecodeString(cfg.EnvironmentDisplayToolchain)
		if err != nil || len(raw) != 32 || cfg.EnvironmentDisplayToolchain != strings.ToLower(cfg.EnvironmentDisplayToolchain) ||
			cfg.EnvironmentDisplayToolchain == cfg.EnvironmentToolchain {
			return fmt.Errorf("geographic display generation requires its own pinned toolchain digest")
		}
	}
	if cfg.EnvironmentLibraryEnabled {
		raw, err := hex.DecodeString(cfg.EnvironmentToolchain)
		if err != nil || len(raw) != 32 || cfg.EnvironmentToolchain != strings.ToLower(cfg.EnvironmentToolchain) || cfg.ArtifactS3Endpoint == "" {
			return fmt.Errorf("geographic environment library requires a pinned toolchain digest and private object storage")
		}
	}
	if cfg.Profile() != ProfileMap && cfg.Profile() != ProfileHistoricalFull {
		return fmt.Errorf("C4ISR_PRODUCT_PROFILE must be map or historical-full")
	}

	environment := strings.ToLower(strings.TrimSpace(cfg.Environment))
	if err := validateEnvironment(environment); err != nil {
		return err
	}
	if cfg.ArtifactS3AllowHTTP && environment == EnvironmentProduction {
		return fmt.Errorf("production object storage requires HTTPS")
	}
	if len(cfg.RuntimeBuilds) > 16 {
		return fmt.Errorf("at most 16 runtime builds may be admitted")
	}
	for _, build := range cfg.RuntimeBuilds {
		if len(build) != 64 {
			return fmt.Errorf("runtime admission requires SHA-256 identities")
		}
		if _, e := hex.DecodeString(build); e != nil {
			return fmt.Errorf("runtime admission requires SHA-256 identities")
		}
	}
	if cfg.ArtifactS3Endpoint != "" || cfg.ArtifactS3Bucket != "" || cfg.ArtifactS3AccessKey != "" || cfg.ArtifactS3SecretKey != "" {
		if cfg.ArtifactS3Endpoint == "" || cfg.ArtifactS3Bucket == "" || cfg.ArtifactS3AccessKey == "" || cfg.ArtifactS3SecretKey == "" {
			return fmt.Errorf("artifact storage requires endpoint, bucket, access key and secret key together")
		}
	}
	for token, operatorID := range cfg.AuthTokens {
		if strings.TrimSpace(token) == "" || strings.TrimSpace(operatorID) == "" {
			return fmt.Errorf("C4ISR_AUTH_TOKENS entries require an operator id and token")
		}
	}
	if !cfg.AuthRequired && environment != EnvironmentDevelopment && environment != EnvironmentTest {
		return fmt.Errorf("C4ISR_AUTH_REQUIRED=false is only permitted in development or test")
	}
	if cfg.AuthRequired && !cfg.DatabaseAuth && len(cfg.AuthTokens) == 0 && environment != EnvironmentDevelopment && environment != EnvironmentTest {
		return fmt.Errorf("C4ISR_AUTH_TOKENS is required when authentication is enabled outside development and test")
	}
	if environment != EnvironmentDevelopment && environment != EnvironmentTest && !strings.EqualFold(strings.TrimSpace(cfg.LogFormat), "json") {
		return fmt.Errorf("C4ISR_LOG_FORMAT=json is required in staging and production")
	}
	return nil
}

// LoadDatabaseURL loads only the database secret. Maintenance tools do not
// require HTTP bearer tokens or unrelated server configuration.
func LoadDatabaseURL() (string, error) {
	url, err := secretEnv("C4ISR_DATABASE_URL")
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", fmt.Errorf("C4ISR_DATABASE_URL or C4ISR_DATABASE_URL_FILE is required")
	}
	return url, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// durationEnv parses a positive duration from the environment, falling back to
// a caller-supplied default when the variable is absent.
func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
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
