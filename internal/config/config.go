// Package config loads and validates application configuration from environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime configuration for the Tackle server.
type Config struct {
	ListenAddr         string
	DatabaseURL        string
	// EncryptionKey is a 64-character hex string representing a 32-byte AES-256 master key.
	// Set via TACKLE_ENCRYPTION_KEY. Never log or expose this value.
	EncryptionKey      string
	Env                string
	// CORSAllowedOrigins is a comma-separated list of allowed CORS origins.
	// Set via TACKLE_CORS_ORIGINS. Defaults to http://localhost:5173 in development.
	CORSAllowedOrigins []string
	// TLSEnabled enables HTTPS on the framework server.
	// Set via TACKLE_TLS_ENABLED. Defaults to false.
	TLSEnabled  bool
	// TLSCertPath is the path to the TLS certificate file.
	// Set via TACKLE_TLS_CERT_PATH.
	TLSCertPath string
	// TLSKeyPath is the path to the TLS private key file.
	// Set via TACKLE_TLS_KEY_PATH.
	TLSKeyPath  string
}

// Load reads configuration from environment variables and returns a validated Config.
// Returns an error if any required variable is missing.
func Load() (*Config, error) {
	corsOrigins := []string{"http://localhost:5173"}
	if v := os.Getenv("TACKLE_CORS_ORIGINS"); v != "" {
		corsOrigins = strings.Split(v, ",")
		for i, o := range corsOrigins {
			corsOrigins[i] = strings.TrimSpace(o)
		}
	}

	tlsEnabled := os.Getenv("TACKLE_TLS_ENABLED") == "true" || os.Getenv("TACKLE_TLS_ENABLED") == "1"

	cfg := &Config{
		ListenAddr:         getEnvOrDefault("TACKLE_LISTEN_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		EncryptionKey:      os.Getenv("TACKLE_ENCRYPTION_KEY"),
		Env:                getEnvOrDefault("TACKLE_ENV", "development"),
		CORSAllowedOrigins: corsOrigins,
		TLSEnabled:         tlsEnabled,
		TLSCertPath:        os.Getenv("TACKLE_TLS_CERT_PATH"),
		TLSKeyPath:         os.Getenv("TACKLE_TLS_KEY_PATH"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("required environment variable DATABASE_URL is not set")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("required environment variable TACKLE_ENCRYPTION_KEY is not set")
	}

	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" {
			return nil, fmt.Errorf("TACKLE_TLS_CERT_PATH is required when TLS is enabled")
		}
		if cfg.TLSKeyPath == "" {
			return nil, fmt.Errorf("TACKLE_TLS_KEY_PATH is required when TLS is enabled")
		}
	}

	return cfg, nil
}

// DecodeEncryptionKey decodes the hex-encoded EncryptionKey into raw bytes.
// TACKLE_ENCRYPTION_KEY must be a 64-character lowercase hex string (32 bytes).
// Returns an error if the value is not valid hex or not exactly 32 bytes.
func (c *Config) DecodeEncryptionKey() ([]byte, error) {
	b, err := hex.DecodeString(c.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("TACKLE_ENCRYPTION_KEY is not valid hex: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("TACKLE_ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(b))
	}
	return b, nil
}

// IsDevelopment reports whether the server is running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Env == "development"
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
