package config

import "testing"

func TestDatabaseCredentialsDoNotRequireStaticOperatorTokens(t *testing.T) {
	t.Setenv("C4ISR_DATABASE_URL", "postgres://fixture/fixture")
	t.Setenv("C4ISR_DATABASE_URL_FILE", "")
	t.Setenv("C4ISR_ENV", "production")
	t.Setenv("C4ISR_LOG_FORMAT", "json")
	t.Setenv("C4ISR_AUTH_REQUIRED", "true")
	t.Setenv("C4ISR_AUTH_TOKENS", "")
	t.Setenv("C4ISR_AUTH_TOKENS_FILE", "")
	t.Setenv("C4ISR_DATABASE_AUTH", "true")
	cfg, err := Load()
	if err != nil || !cfg.AuthRequired || !cfg.DatabaseAuth || len(cfg.AuthTokens) != 0 {
		t.Fatalf("database authentication configuration: %+v (%v)", cfg, err)
	}
	t.Setenv("C4ISR_AUTH_REQUIRED", "false")
	if _, err = Load(); err == nil {
		t.Fatal("database authentication allowed unauthenticated production requests")
	}
	t.Setenv("C4ISR_AUTH_REQUIRED", "true")
	t.Setenv("C4ISR_DATABASE_AUTH", "invalid")
	if _, err = Load(); err == nil {
		t.Fatal("invalid database authentication flag accepted")
	}
}
