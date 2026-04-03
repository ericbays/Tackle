package crypto

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// encryptedColumn describes a table and column that stores AES-256-GCM ciphertext.
type encryptedColumn struct {
	table  string
	column string
	idCol  string // primary key column name
}

// allEncryptedColumns lists every table/column pair that stores AES-256-GCM
// ciphertext and must be re-encrypted during key rotation.
var allEncryptedColumns = []encryptedColumn{
	// Phase 1 originals.
	{table: "auth_providers", column: "configuration", idCol: "id"},
	{table: "webhook_endpoints", column: "auth_config", idCol: "id"},
	{table: "notification_smtp_config", column: "password", idCol: "id"},
	{table: "ai_providers", column: "configuration", idCol: "id"},

	// SMTP profiles (Phase 2).
	{table: "smtp_profiles", column: "password_encrypted", idCol: "id"},
	{table: "smtp_profiles", column: "username_encrypted", idCol: "id"},

	// Domain provider connections (Phase 2).
	{table: "domain_provider_connections", column: "credentials_encrypted", idCol: "id"},

	// Cloud credentials (Phase 2).
	{table: "cloud_credentials", column: "credentials_encrypted", idCol: "id"},

	// Credential capture fields (Phase 3).
	{table: "capture_fields", column: "field_value_encrypted", idCol: "id"},

	// Session captures (Phase 3).
	{table: "session_captures", column: "key_encrypted", idCol: "id"},
	{table: "session_captures", column: "value_encrypted", idCol: "id"},

	// Endpoint SSH keys (Phase 3).
	{table: "endpoint_ssh_keys", column: "private_key_encrypted", idCol: "id"},

	// DKIM signing keys (Phase 2).
	{table: "dkim_keys", column: "private_key_encrypted", idCol: "id"},
}

// Rotator re-encrypts all encrypted database columns from one key to another.
type Rotator struct {
	db     *sql.DB
	oldSvc *EncryptionService
	newSvc *EncryptionService
	log    *slog.Logger
}

// NewRotator creates a Rotator that will decrypt with oldSvc and re-encrypt with newSvc.
func NewRotator(db *sql.DB, oldSvc, newSvc *EncryptionService, log *slog.Logger) *Rotator {
	return &Rotator{db: db, oldSvc: oldSvc, newSvc: newSvc, log: log}
}

// Rotate re-encrypts all encrypted columns. It is idempotent: if oldSvc
// and newSvc use the same key the function detects this and skips all work.
//
// Each row is processed inside its own transaction so the rotation can be safely
// resumed if interrupted midway.
func (r *Rotator) Rotate(ctx context.Context) error {
	if bytes.Equal(r.oldSvc.key, r.newSvc.key) {
		r.log.Info("key rotation skipped: old and new keys are identical")
		return nil
	}

	for _, col := range allEncryptedColumns {
		if err := r.rotateColumn(ctx, col); err != nil {
			return fmt.Errorf("rotate %s.%s: %w", col.table, col.column, err)
		}
	}
	return nil
}

func (r *Rotator) rotateColumn(ctx context.Context, col encryptedColumn) error {
	// #nosec G201 — table/column names come from the hard-coded allEncryptedColumns list,
	// not from user input. Parameterized queries cannot be used for identifiers.
	selectSQL := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s IS NOT NULL",
		col.idCol, col.column, col.table, col.column,
	)

	rows, err := r.db.QueryContext(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("select: %w", err)
	}
	defer rows.Close()

	type rowData struct {
		id         string
		ciphertext []byte
	}
	var batch []rowData

	for rows.Next() {
		var rd rowData
		if err := rows.Scan(&rd.id, &rd.ciphertext); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		batch = append(batch, rd)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}

	processed := 0
	for _, rd := range batch {
		plaintext, err := r.oldSvc.Decrypt(rd.ciphertext)
		if err != nil {
			return fmt.Errorf("decrypt row %s: %w", rd.id, err)
		}
		newCiphertext, err := r.newSvc.Encrypt(plaintext)
		if err != nil {
			return fmt.Errorf("re-encrypt row %s: %w", rd.id, err)
		}

		// #nosec G201 — same justification as above.
		updateSQL := fmt.Sprintf(
			"UPDATE %s SET %s = $1 WHERE %s = $2",
			col.table, col.column, col.idCol,
		)
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for row %s: %w", rd.id, err)
		}
		if _, err := tx.ExecContext(ctx, updateSQL, newCiphertext, rd.id); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update row %s: %w", rd.id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit row %s: %w", rd.id, err)
		}
		processed++
	}

	r.log.Info("key rotation complete for column",
		slog.String("table", col.table),
		slog.String("column", col.column),
		slog.Int("rows_processed", processed),
	)
	return nil
}
