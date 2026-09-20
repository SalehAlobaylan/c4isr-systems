package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAuthTokens(t *testing.T) {
	got, err := parseAuthTokens("operator-01=one, supervisor-01=two")
	if err != nil {
		t.Fatal(err)
	}
	if got["one"] != "operator-01" || got["two"] != "supervisor-01" {
		t.Fatalf("parsed tokens = %#v", got)
	}
	if _, err := parseAuthTokens("missing-separator"); err == nil {
		t.Fatal("malformed token entry should fail")
	}
}

func TestParseBool(t *testing.T) {
	for _, raw := range []string{"true", "1", "yes", "on"} {
		got, err := parseBool("TEST", raw)
		if err != nil || !got {
			t.Errorf("parseBool(%q) = %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"false", "0", "no", "off"} {
		got, err := parseBool("TEST", raw)
		if err != nil || got {
			t.Errorf("parseBool(%q) = %v, %v", raw, got, err)
		}
	}
}

func TestValidateEnvironmentAuthenticationSafety(t *testing.T) {
	if err := (Config{Environment: EnvironmentProduction, AuthRequired: false}).Validate(); err == nil {
		t.Fatal("production must reject disabled authentication")
	}
	if err := (Config{Environment: EnvironmentProduction, AuthRequired: true}).Validate(); err == nil {
		t.Fatal("production must reject missing authentication tokens")
	}
	if err := (Config{Environment: EnvironmentDevelopment, AuthRequired: false}).Validate(); err != nil {
		t.Fatalf("development should allow disabled authentication: %v", err)
	}
	if err := (Config{Environment: EnvironmentTest, AuthRequired: false}).Validate(); err != nil {
		t.Fatalf("test should allow disabled authentication: %v", err)
	}
	if err := (Config{Environment: "unknown", AuthRequired: true, AuthTokens: map[string]string{"token": "operator-01"}}).Validate(); err == nil {
		t.Fatal("unknown environment should fail")
	}
	if err := (Config{Environment: EnvironmentStaging, AuthRequired: true, LogFormat: "json", AuthTokens: map[string]string{"token": "operator-01"}}).Validate(); err != nil {
		t.Fatalf("staging should allow production-safe authentication: %v", err)
	}
	if err := (Config{Environment: EnvironmentStaging, AuthRequired: true, LogFormat: "text", AuthTokens: map[string]string{"token": "operator-01"}}).Validate(); err == nil {
		t.Fatal("staging should require JSON logs")
	}
	if err := (Config{Environment: EnvironmentProduction, AuthRequired: true, AuthTokens: map[string]string{"": "operator-01"}}).Validate(); err == nil {
		t.Fatal("empty configured token should fail")
	}
}

func TestLoadDevelopmentInstallsOnlyExplicitDevelopmentFallback(t *testing.T) {
	t.Setenv("C4ISR_DATABASE_URL", "postgres://example")
	t.Setenv("C4ISR_AUTH_REQUIRED", "true")
	t.Setenv("C4ISR_AUTH_TOKENS", "")
	t.Setenv("C4ISR_ENV", EnvironmentDevelopment)
	dev, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(dev.AuthTokens) != 1 || dev.AuthTokens["dev-operator-token"] != "operator-01" {
		t.Fatalf("development fallback = %#v", dev.AuthTokens)
	}

	t.Setenv("C4ISR_ENV", EnvironmentProduction)
	if _, err := Load(); err == nil {
		t.Fatal("production without configured tokens should fail")
	}

	t.Setenv("C4ISR_ENV", EnvironmentTest)
	if testCfg, err := Load(); err != nil {
		t.Fatalf("test mode may omit tokens for isolated harnesses: %v", err)
	} else if len(testCfg.AuthTokens) != 0 {
		t.Fatalf("test mode unexpectedly installed tokens: %#v", testCfg.AuthTokens)
	}
}

func TestLoadReadsFileBackedSecrets(t *testing.T) {
	dir := t.TempDir()
	databaseFile := filepath.Join(dir, "database-url")
	tokensFile := filepath.Join(dir, "auth-tokens")
	if err := os.WriteFile(databaseFile, []byte("postgres://file-example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokensFile, []byte("operator-01=file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("C4ISR_DATABASE_URL", "")
	t.Setenv("C4ISR_DATABASE_URL_FILE", databaseFile)
	t.Setenv("C4ISR_AUTH_TOKENS", "")
	t.Setenv("C4ISR_AUTH_TOKENS_FILE", tokensFile)
	t.Setenv("C4ISR_ENV", EnvironmentStaging)
	t.Setenv("C4ISR_LOG_FORMAT", "json")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://file-example" || cfg.AuthTokens["file-token"] != "operator-01" {
		t.Fatalf("file-backed secrets were not loaded: %#v", cfg)
	}
}

func TestLoadRejectsInlineAndFileSecretTogether(t *testing.T) {
	dir := t.TempDir()
	databaseFile := filepath.Join(dir, "database-url")
	if err := os.WriteFile(databaseFile, []byte("postgres://file-example"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("C4ISR_DATABASE_URL", "postgres://inline-example")
	t.Setenv("C4ISR_DATABASE_URL_FILE", databaseFile)
	t.Setenv("C4ISR_AUTH_TOKENS", "")
	t.Setenv("C4ISR_AUTH_TOKENS_FILE", "")
	t.Setenv("C4ISR_ENV", EnvironmentDevelopment)
	if _, err := Load(); err == nil {
		t.Fatal("inline and file-backed database secrets should not be accepted together")
	}
}
