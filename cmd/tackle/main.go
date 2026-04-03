// Command tackle is the entry point for the Tackle phishing simulation platform.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"

	"tackle/internal/config"
	"tackle/internal/crypto"
	"tackle/internal/database"
	"tackle/internal/logger"
	"tackle/internal/migrations"
	"tackle/internal/server"
	auditsvc "tackle/internal/services/audit"
	"tackle/internal/workers"
)

func main() {
	migrateUp := flag.Bool("migrate-up", false, "apply all pending migrations and exit")
	migrateDown := flag.Bool("migrate-down", false, "roll back the last migration and exit")
	rotateKeyOld := flag.String("rotate-key-old", "", "old 32-byte master key (64-char hex) for key rotation")
	rotateKeyNew := flag.String("rotate-key-new", "", "new 32-byte master key (64-char hex) for key rotation")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log := logger.New(cfg.IsDevelopment())

	// Decode and validate the master encryption key.
	// TACKLE_ENCRYPTION_KEY must be a 64-character hex string (32 bytes).
	masterKey, err := cfg.DecodeEncryptionKey()
	if err != nil {
		log.Error("invalid encryption key", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Derive the DB encryption subkey and build the shared EncryptionService.
	encSvc, err := crypto.NewEncryptionServiceForPurpose(masterKey, crypto.PurposeDBEncryption)
	if err != nil {
		log.Error("failed to initialise encryption service", slog.String("error", err.Error()))
		os.Exit(1)
	}
	_ = encSvc // threaded into server components in subsequent sessions

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	migrationsPath := resolveMigrationsPath()

	if *migrateUp {
		if err := migrations.RunUp(db, migrationsPath, log); err != nil {
			log.Error("migrate up failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}

	if *migrateDown {
		if err := migrations.RunDown(db, migrationsPath, log); err != nil {
			log.Error("migrate down failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		return
	}

	// Key rotation mode: re-encrypt all Phase 1 encrypted columns.
	// Usage: tackle -rotate-key-old <hex> -rotate-key-new <hex>
	if *rotateKeyOld != "" || *rotateKeyNew != "" {
		if *rotateKeyOld == "" || *rotateKeyNew == "" {
			log.Error("both -rotate-key-old and -rotate-key-new must be provided")
			os.Exit(1)
		}

		oldBytes, err := hex.DecodeString(*rotateKeyOld)
		if err != nil || len(oldBytes) != 32 {
			log.Error("-rotate-key-old must be a 64-character hex string (32 bytes)")
			os.Exit(1)
		}
		newBytes, err := hex.DecodeString(*rotateKeyNew)
		if err != nil || len(newBytes) != 32 {
			log.Error("-rotate-key-new must be a 64-character hex string (32 bytes)")
			os.Exit(1)
		}

		oldSvc, err := crypto.NewEncryptionServiceForPurpose(oldBytes, crypto.PurposeDBEncryption)
		if err != nil {
			log.Error("failed to build old encryption service", slog.String("error", err.Error()))
			os.Exit(1)
		}
		newSvc, err := crypto.NewEncryptionServiceForPurpose(newBytes, crypto.PurposeDBEncryption)
		if err != nil {
			log.Error("failed to build new encryption service", slog.String("error", err.Error()))
			os.Exit(1)
		}

		rotator := crypto.NewRotator(db, oldSvc, newSvc, log)
		if err := rotator.Rotate(context.Background()); err != nil {
			log.Error("key rotation failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		log.Info("key rotation completed successfully")
		return
	}

	srv, auditSvc := server.New(cfg, db, masterKey, log)

	// Set migration callback so migrations are audit-logged.
	migrations.SetMigrationCallback(func(version uint, direction string) {
		_ = auditSvc.Log(context.Background(), auditsvc.LogEntry{
			Category:   auditsvc.CategorySystem,
			Severity:   auditsvc.SeverityInfo,
			ActorType:  auditsvc.ActorTypeSystem,
			ActorLabel: "system",
			Action:     "system.migration",
			Details:    map[string]any{"version": version, "direction": direction},
		})
	})

	// Auto-apply migrations on every startup.
	if err := migrations.RunUp(db, migrationsPath, log); err != nil {
		log.Error("startup migrations failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Start background workers.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	notifCleanup := workers.NewNotificationCleanupWorker(db, log)
	go notifCleanup.Start(workerCtx)

	// Wrap server run with panic recovery and system event logging.
	runServer := func() error {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				_ = auditSvc.Log(context.Background(), auditsvc.LogEntry{
					Category:   auditsvc.CategorySystem,
					Severity:   auditsvc.SeverityCritical,
					ActorType:  auditsvc.ActorTypeSystem,
					ActorLabel: "system",
					Action:     "system.panic",
					Details:    map[string]any{"panic": fmt.Sprintf("%v", r), "stack": stack},
				})
				auditSvc.Drain()
				log.Error("panic recovered", slog.String("error", fmt.Sprintf("%v", r)))
				os.Exit(1)
			}
		}()

		// Log startup event.
		_ = auditSvc.Log(context.Background(), auditsvc.LogEntry{
			Category:   auditsvc.CategorySystem,
			Severity:   auditsvc.SeverityInfo,
			ActorType:  auditsvc.ActorTypeSystem,
			ActorLabel: "system",
			Action:     "system.startup",
			Details:    map[string]any{"version": "dev", "addr": cfg.ListenAddr},
		})

		err := server.Run(srv, cfg, log)

		// Log shutdown event (best-effort before drain).
		_ = auditSvc.Log(context.Background(), auditsvc.LogEntry{
			Category:   auditsvc.CategorySystem,
			Severity:   auditsvc.SeverityInfo,
			ActorType:  auditsvc.ActorTypeSystem,
			ActorLabel: "system",
			Action:     "system.shutdown",
		})

		return err
	}

	if err := runServer(); err != nil {
		log.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// resolveMigrationsPath returns the path to the migrations directory.
// Checks TACKLE_MIGRATIONS_PATH env var first, then falls back to "migrations"
// relative to the working directory (which is the project root for go run and
// the binary's install directory for production deployments).
func resolveMigrationsPath() string {
	if v := os.Getenv("TACKLE_MIGRATIONS_PATH"); v != "" {
		return v
	}
	return filepath.Join(".", "migrations")
}
