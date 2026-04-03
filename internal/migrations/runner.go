// Package migrations provides helpers for running database schema migrations
// using golang-migrate against the Tackle PostgreSQL database.
package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrationCallback is called after a migration completes with version and direction.
type MigrationCallback func(version uint, direction string)

// migrationCallback is an optional hook called after successful migrations.
var migrationCallback MigrationCallback

// SetMigrationCallback sets a callback that is invoked after each successful migration.
func SetMigrationCallback(cb MigrationCallback) {
	migrationCallback = cb
}

// RunUp applies all pending migrations found at migrationsPath to the database
// connected via db. It logs each applied migration using logger.
// Returns nil if no pending migrations exist (already up to date).
func RunUp(db *sql.DB, migrationsPath string, logger *slog.Logger) error {
	m, err := newMigrate(db, migrationsPath)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("migrations: already up to date")
			return nil
		}
		return fmt.Errorf("migrate up: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("migrate version: %w", err)
	}
	logger.Info("migrations: applied successfully",
		slog.Uint64("version", uint64(version)),
		slog.Bool("dirty", dirty),
	)
	if migrationCallback != nil {
		migrationCallback(version, "up")
	}
	return nil
}

// RunDown rolls back the most recent migration applied to the database.
// Returns nil if no migrations have been applied yet.
func RunDown(db *sql.DB, migrationsPath string, logger *slog.Logger) error {
	m, err := newMigrate(db, migrationsPath)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}

	if err := m.Steps(-1); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("migrations: nothing to roll back")
			return nil
		}
		return fmt.Errorf("migrate down: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("migrate version: %w", err)
	}
	logger.Info("migrations: rolled back one step",
		slog.Uint64("version", uint64(version)),
		slog.Bool("dirty", dirty),
	)
	if migrationCallback != nil {
		migrationCallback(version, "down")
	}
	return nil
}

// newMigrate constructs a *migrate.Migrate instance wired to the given *sql.DB
// and a file-based source at migrationsPath.
// Uses iofs.New with os.DirFS to avoid golang-migrate's Windows file:// URL bug.
func newMigrate(db *sql.DB, migrationsPath string) (*migrate.Migrate, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("postgres driver: %w", err)
	}

	src, err := iofs.New(os.DirFS(migrationsPath), ".")
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("new migrate instance: %w", err)
	}

	return m, nil
}
