//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"tackle/internal/config"
	"tackle/internal/server"
)

// newTestLogger returns a slog.Logger that writes to the test log.
func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

type testWriter struct{ t *testing.T }

func (tw testWriter) Write(p []byte) (int, error) {
	tw.t.Log(string(p))
	return len(p), nil
}

// newBenchLogger returns a discard logger for benchmarks.
func newBenchLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// getTestDBURL returns TEST_DATABASE_URL or skips the benchmark.
func getTestDBURL(b *testing.B) string {
	b.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		b.Skip("TEST_DATABASE_URL not set — skipping benchmark")
	}
	return u
}

// openTestDB opens and pings a PostgreSQL connection.
func openTestDB(b *testing.B, url string) *sql.DB {
	b.Helper()
	db, err := sql.Open("postgres", url)
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		b.Fatalf("ping db: %v", err)
	}
	return db
}

// testMasterKey returns a deterministic 32-byte key for tests.
func testMasterKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

// testConfig returns a minimal Config for tests.
func testConfig(dbURL string) *config.Config {
	return &config.Config{
		ListenAddr:         ":0",
		DatabaseURL:        dbURL,
		Env:                "test",
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}
}

// buildTestServer creates a test HTTP server and registers cleanup.
func buildTestServer(b *testing.B, cfg *config.Config, db *sql.DB, masterKey []byte, logger *slog.Logger) *httptest.Server {
	b.Helper()
	httpSrv := server.New(cfg, db, masterKey, logger)
	ts := httptest.NewServer(httpSrv.Handler)
	b.Cleanup(ts.Close)
	return ts
}

// resetDB removes all user/session/audit data for a clean test run.
func resetDB(tb testing.TB, db *sql.DB) {
	tb.Helper()
	tables := []string{
		"audit_logs", "sessions", "user_roles", "users",
	}
	for _, tbl := range tables {
		if _, err := db.ExecContext(context.Background(), "DELETE FROM "+tbl); err != nil {
			tb.Logf("resetDB: delete from %s: %v (may not exist)", tbl, err)
		}
	}
}

// mustSetupB runs the setup wizard for benchmarks.
func mustSetupB(b *testing.B, env *testEnv) string {
	b.Helper()
	resp := env.do("POST", "/setup", map[string]string{
		"username":              "admin",
		"email":                 "admin@example.com",
		"display_name":          "Administrator",
		"password":              "S3cur3P@ssw0rd!XYZ",
		"password_confirmation": "S3cur3P@ssw0rd!XYZ",
	}, "")
	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		b.Fatalf("mustSetupB: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var sr struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	defer resp.Body.Close()
	json.NewDecoder(resp.Body).Decode(&sr) //nolint:errcheck
	return sr.Data.AccessToken
}

// decode reads and JSON-decodes a response body.
func decode(t *testing.T, resp interface{ Body io.ReadCloser }, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// readAll reads a response body and returns it as bytes.
func readAll(resp interface{ Body io.ReadCloser }) []byte {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b
}

var _ = bytes.Contains // imported for integration_test.go use
