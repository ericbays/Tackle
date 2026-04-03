// Package database provides PostgreSQL connection management for Tackle.
package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

// Connect opens a PostgreSQL connection pool, applies pool configuration,
// and verifies connectivity via a ping. The caller is responsible for
// closing the returned *sql.DB when it is no longer needed.
//
// TLS enforcement: if TACKLE_DB_SSLMODE is not set, sslmode=require is
// injected into the DSN. Set TACKLE_DB_SSLMODE=disable to opt out (a
// warning is logged). TACKLE_DB_SSLROOTCERT specifies a CA certificate path.
//
// Pool settings are configurable via environment variables:
//   - TACKLE_DB_MAX_OPEN_CONNS (default 25)
//   - TACKLE_DB_MAX_IDLE_CONNS (default 10)
//   - TACKLE_DB_CONN_MAX_LIFETIME (default "30m", Go duration string)
func Connect(databaseURL string) (*sql.DB, error) {
	databaseURL = enforceDBTLS(databaseURL)

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	maxOpen := envInt("TACKLE_DB_MAX_OPEN_CONNS", 25)
	maxIdle := envInt("TACKLE_DB_MAX_IDLE_CONNS", 10)
	maxLifetime := envDuration("TACKLE_DB_CONN_MAX_LIFETIME", 30*time.Minute)

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// enforceDBTLS ensures the database connection uses TLS by default.
// If TACKLE_DB_SSLMODE is set, it overrides whatever is in the DSN.
// If neither the env var nor the DSN contains sslmode, sslmode=require is added.
// A warning is logged when sslmode=disable is used.
func enforceDBTLS(dsn string) string {
	sslMode := os.Getenv("TACKLE_DB_SSLMODE")
	sslRootCert := os.Getenv("TACKLE_DB_SSLROOTCERT")

	u, err := url.Parse(dsn)
	if err != nil {
		// If DSN isn't a URL (e.g. key=value format), fall through to simple append.
		if sslMode == "" {
			sslMode = "require"
		}
		if sslMode == "disable" {
			slog.Warn("database TLS is disabled — connections are unencrypted")
		}
		// For key=value DSN, just append parameters.
		dsn = dsn + " sslmode=" + sslMode
		if sslRootCert != "" {
			dsn = dsn + " sslrootcert=" + sslRootCert
		}
		return dsn
	}

	q := u.Query()
	if sslMode != "" {
		q.Set("sslmode", sslMode)
	} else if q.Get("sslmode") == "" {
		q.Set("sslmode", "require")
	}

	if sslRootCert != "" {
		q.Set("sslrootcert", sslRootCert)
	}

	if q.Get("sslmode") == "disable" {
		slog.Warn("database TLS is disabled — connections are unencrypted")
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// envInt reads an integer from an environment variable, returning def if unset or invalid.
func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// envDuration reads a Go duration string from an environment variable, returning def if unset or invalid.
func envDuration(key string, def time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return v
}
