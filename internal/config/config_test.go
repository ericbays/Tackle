package config

import (
	"os"
	"testing"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestLoad_TLSEnabled_RequiresCertAndKey(t *testing.T) {
	// Set required env vars.
	setEnv(t, "DATABASE_URL", "postgres://localhost/tackle_test")
	setEnv(t, "TACKLE_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	// TLS enabled without cert path → error.
	setEnv(t, "TACKLE_TLS_ENABLED", "true")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TLS enabled without cert path")
	}

	// TLS enabled with cert but no key → error.
	setEnv(t, "TACKLE_TLS_CERT_PATH", "/path/to/cert.pem")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error when TLS enabled without key path")
	}

	// TLS enabled with both → success.
	setEnv(t, "TACKLE_TLS_KEY_PATH", "/path/to/key.pem")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.TLSEnabled {
		t.Error("expected TLSEnabled=true")
	}
	if cfg.TLSCertPath != "/path/to/cert.pem" {
		t.Errorf("unexpected TLSCertPath: %s", cfg.TLSCertPath)
	}
}

func TestLoad_TLSDisabled_NoCertRequired(t *testing.T) {
	setEnv(t, "DATABASE_URL", "postgres://localhost/tackle_test")
	setEnv(t, "TACKLE_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	// Ensure TLS env vars are not set.
	os.Unsetenv("TACKLE_TLS_ENABLED")
	os.Unsetenv("TACKLE_TLS_CERT_PATH")
	os.Unsetenv("TACKLE_TLS_KEY_PATH")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TLSEnabled {
		t.Error("expected TLSEnabled=false by default")
	}
}
