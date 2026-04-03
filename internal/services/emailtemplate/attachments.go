// Package emailtemplate — attachments provides email template attachment management.
package emailtemplate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Attachment is the DB model for an email_template_attachments row.
type Attachment struct {
	ID            string
	TemplateID    string
	Filename      string
	ContentType   string
	FileSizeBytes int64
	StoragePath   string
	IsInline      bool
	ContentID     *string
	CreatedAt     time.Time
}

// AttachmentDTO is the API representation of an attachment.
type AttachmentDTO struct {
	ID            string  `json:"id"`
	TemplateID    string  `json:"template_id"`
	Filename      string  `json:"filename"`
	ContentType   string  `json:"content_type"`
	FileSizeBytes int64   `json:"file_size_bytes"`
	IsInline      bool    `json:"is_inline"`
	ContentID     *string `json:"content_id,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

// AllowedMIMETypes is the default allowlist for attachment MIME types.
var AllowedMIMETypes = map[string]bool{
	"application/pdf":       true,
	"image/png":             true,
	"image/jpeg":            true,
	"image/gif":             true,
	"application/msword":    true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"text/plain": true,
	"text/csv":   true,
}

// MaxAttachmentSize is the default max file size (10 MB).
const MaxAttachmentSize = 10 * 1024 * 1024

// AttachmentService manages email template attachments.
type AttachmentService struct {
	db         *sql.DB
	storageDir string
}

// NewAttachmentService creates a new AttachmentService.
func NewAttachmentService(db *sql.DB, storageDir string) *AttachmentService {
	return &AttachmentService{db: db, storageDir: storageDir}
}

// Upload stores an attachment file and creates a DB record.
func (s *AttachmentService) Upload(ctx context.Context, templateID string, filename string, contentType string, size int64, reader io.Reader) (AttachmentDTO, error) {
	// Validate MIME type.
	if !AllowedMIMETypes[strings.ToLower(contentType)] {
		return AttachmentDTO{}, fmt.Errorf("MIME type %q is not allowed", contentType)
	}

	// Validate size.
	if size > MaxAttachmentSize {
		return AttachmentDTO{}, fmt.Errorf("file size %d exceeds maximum of %d bytes", size, MaxAttachmentSize)
	}

	// Generate storage path.
	id := uuid.New().String()
	dir := filepath.Join(s.storageDir, "attachments", templateID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return AttachmentDTO{}, fmt.Errorf("create storage dir: %w", err)
	}

	// Sanitize filename.
	safeName := filepath.Base(filename)
	storagePath := filepath.Join(dir, id+"_"+safeName)

	// Write file.
	f, err := os.Create(storagePath)
	if err != nil {
		return AttachmentDTO{}, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(reader, MaxAttachmentSize+1))
	if err != nil {
		_ = os.Remove(storagePath)
		return AttachmentDTO{}, fmt.Errorf("write file: %w", err)
	}
	if written > MaxAttachmentSize {
		_ = os.Remove(storagePath)
		return AttachmentDTO{}, fmt.Errorf("file size exceeds maximum of %d bytes", MaxAttachmentSize)
	}

	// Insert DB record.
	const q = `
		INSERT INTO email_template_attachments (id, template_id, filename, content_type, file_size_bytes, storage_path)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, template_id, filename, content_type, file_size_bytes, storage_path, is_inline, content_id, created_at`
	var a Attachment
	err = s.db.QueryRowContext(ctx, q, id, templateID, safeName, contentType, written, storagePath).Scan(
		&a.ID, &a.TemplateID, &a.Filename, &a.ContentType, &a.FileSizeBytes,
		&a.StoragePath, &a.IsInline, &a.ContentID, &a.CreatedAt,
	)
	if err != nil {
		_ = os.Remove(storagePath)
		return AttachmentDTO{}, fmt.Errorf("insert attachment: %w", err)
	}

	return toAttachmentDTO(a), nil
}

// List returns all attachments for a template.
func (s *AttachmentService) List(ctx context.Context, templateID string) ([]AttachmentDTO, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, template_id, filename, content_type, file_size_bytes, storage_path, is_inline, content_id, created_at
		 FROM email_template_attachments WHERE template_id = $1 ORDER BY created_at ASC`, templateID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var result []AttachmentDTO
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.TemplateID, &a.Filename, &a.ContentType, &a.FileSizeBytes,
			&a.StoragePath, &a.IsInline, &a.ContentID, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		result = append(result, toAttachmentDTO(a))
	}
	if result == nil {
		result = []AttachmentDTO{}
	}
	return result, nil
}

// Delete removes an attachment record and its file.
func (s *AttachmentService) Delete(ctx context.Context, attachmentID string) error {
	// Get storage path first.
	var storagePath string
	err := s.db.QueryRowContext(ctx,
		`SELECT storage_path FROM email_template_attachments WHERE id = $1`, attachmentID,
	).Scan(&storagePath)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("attachment not found")
		}
		return fmt.Errorf("get attachment: %w", err)
	}

	// Delete DB record.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM email_template_attachments WHERE id = $1`, attachmentID); err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}

	// Remove file (best effort).
	_ = os.Remove(storagePath)
	return nil
}

func toAttachmentDTO(a Attachment) AttachmentDTO {
	return AttachmentDTO{
		ID:            a.ID,
		TemplateID:    a.TemplateID,
		Filename:      a.Filename,
		ContentType:   a.ContentType,
		FileSizeBytes: a.FileSizeBytes,
		IsInline:      a.IsInline,
		ContentID:     a.ContentID,
		CreatedAt:     a.CreatedAt.Format(time.RFC3339),
	}
}
