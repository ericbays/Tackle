package migrations_test

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/lib/pq"

	"tackle/internal/database"
	"tackle/internal/logger"
	"tackle/internal/migrations"
)

// migrationsDir returns the absolute path to the project migrations directory.
func migrationsDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/migrations/runner_test.go → root is two levels up.
	root := filepath.Join(filepath.Dir(filename), "..", "..")
	return filepath.Join(root, "migrations")
}

// openTestDB opens a *sql.DB from DATABASE_URL. Skips the test if the
// variable is not set so CI runs without a live database still compile.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := database.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testLogger(t *testing.T) *slog.Logger { //nolint:thelper
	t.Helper()
	return logger.New(true)
}

// TestMigrateUpDown verifies that all migrations apply to a clean database and
// then roll back completely without error.
func TestMigrateUpDown(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}

	// Roll back all 15 migrations one at a time.
	for i := 0; i < 15; i++ {
		if err := migrations.RunDown(db, dir, log); err != nil {
			t.Fatalf("RunDown step %d: %v", i+1, err)
		}
	}

	// A second RunDown on a clean database should be a no-op.
	if err := migrations.RunDown(db, dir, log); err != nil {
		t.Fatalf("RunDown on empty schema: %v", err)
	}
}

// TestBuiltinRoles verifies that the four built-in roles are seeded with
// is_builtin = true after RunUp.
func TestBuiltinRoles(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	rows, err := db.Query(`SELECT name FROM roles WHERE is_builtin = TRUE ORDER BY name`)
	if err != nil {
		t.Fatalf("query roles: %v", err)
	}
	defer rows.Close()

	want := map[string]bool{"admin": false, "defender": false, "engineer": false, "operator": false}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected builtin role: %q", name)
			continue
		}
		want[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	for name, found := range want {
		if !found {
			t.Errorf("builtin role %q not found", name)
		}
	}
}

// TestPermissionMatrixPopulated verifies that permissions are seeded for each
// known resource type.
func TestPermissionMatrixPopulated(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	expectedResources := []string{
		"users", "roles", "campaigns", "targets",
		"templates.email", "templates.landing",
		"domains", "endpoints", "smtp", "credentials",
		"reports", "metrics",
		"logs.audit", "logs.campaign", "logs.system",
		"settings", "settings.auth",
		"cloud", "infrastructure.requests", "schedules",
		"notifications", "api_keys",
	}

	for _, resource := range expectedResources {
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM permissions WHERE resource_type = $1`, resource,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query permissions for %q: %v", resource, err)
		}
		if count == 0 {
			t.Errorf("no permissions seeded for resource_type %q", resource)
		}
	}
}

// TestAuditLogsImmutableUpdate verifies the trigger rejects UPDATE on audit_logs.
func TestAuditLogsImmutableUpdate(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	// Insert a row into the current partition.
	_, err := db.Exec(`
		INSERT INTO audit_logs (timestamp, category, severity, actor_type, action)
		VALUES (now(), 'system', 'info', 'system', 'test.insert')
	`)
	if err != nil {
		t.Fatalf("insert audit log: %v", err)
	}

	// Attempt UPDATE — must fail.
	_, err = db.Exec(`UPDATE audit_logs SET action = 'tampered' WHERE action = 'test.insert'`)
	if err == nil {
		t.Fatal("expected UPDATE on audit_logs to fail, but it succeeded")
	}
}

// TestAuditLogsImmutableDelete verifies the trigger rejects DELETE on audit_logs.
func TestAuditLogsImmutableDelete(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	_, err := db.Exec(`
		INSERT INTO audit_logs (timestamp, category, severity, actor_type, action)
		VALUES (now(), 'system', 'info', 'system', 'test.delete.target')
	`)
	if err != nil {
		t.Fatalf("insert audit log: %v", err)
	}

	_, err = db.Exec(`DELETE FROM audit_logs WHERE action = 'test.delete.target'`)
	if err == nil {
		t.Fatal("expected DELETE on audit_logs to fail, but it succeeded")
	}
}

// TestSetUpdatedAtTrigger verifies the updated_at trigger fires on UPDATE.
func TestSetUpdatedAtTrigger(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	// Insert a role with a known updated_at.
	var id string
	err := db.QueryRow(`
		INSERT INTO roles (name, description)
		VALUES ('test-trigger-role', 'trigger test')
		RETURNING id
	`).Scan(&id)
	if err != nil {
		t.Fatalf("insert role: %v", err)
	}

	var before, after string
	if err := db.QueryRow(`SELECT updated_at FROM roles WHERE id = $1`, id).Scan(&before); err != nil {
		t.Fatalf("read updated_at before: %v", err)
	}

	// Sleep a tiny bit so clock advances, then update.
	_, err = db.Exec(`
		UPDATE roles SET description = 'trigger test updated'
		WHERE id = $1
	`, id)
	if err != nil {
		t.Fatalf("update role: %v", err)
	}

	if err := db.QueryRow(`SELECT updated_at FROM roles WHERE id = $1`, id).Scan(&after); err != nil {
		t.Fatalf("read updated_at after: %v", err)
	}

	if before == after {
		t.Error("expected updated_at to change after UPDATE, but it did not")
	}

	// Cleanup.
	db.Exec(`DELETE FROM roles WHERE id = $1`, id) //nolint:errcheck
}

// TestCheckConstraintRejectsInvalidEnum verifies CHECK constraints refuse bad values.
func TestCheckConstraintRejectsInvalidEnum(t *testing.T) {
	db := openTestDB(t)
	log := testLogger(t)
	dir := migrationsDir(t)

	if err := migrations.RunUp(db, dir, log); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	t.Cleanup(func() { rollbackAll(t, db, dir, log) })

	// users.status CHECK constraint.
	_, err := db.Exec(`
		INSERT INTO users (email, username, display_name, status)
		VALUES ('bad@example.com', 'baduser', 'Bad User', 'suspended')
	`)
	if err == nil {
		t.Error("expected CHECK constraint violation for status='suspended', got nil")
	}

	// audit_logs category CHECK constraint.
	_, err = db.Exec(`
		INSERT INTO audit_logs (timestamp, category, severity, actor_type, action)
		VALUES (now(), 'unknown_category', 'info', 'system', 'test')
	`)
	if err == nil {
		t.Error("expected CHECK constraint violation for category='unknown_category', got nil")
	}
}

// rollbackAll rolls back all 15 migrations in order for post-test cleanup.
func rollbackAll(t *testing.T, db *sql.DB, dir string, log *slog.Logger) {
	t.Helper()
	for i := 0; i < 15; i++ {
		if err := migrations.RunDown(db, dir, log); err != nil {
			t.Logf("rollbackAll step %d: %v", i+1, err)
			return
		}
	}
}
