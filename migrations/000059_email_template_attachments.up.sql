-- Migration 059: email template attachments.

CREATE TABLE email_template_attachments (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id   UUID        NOT NULL REFERENCES email_templates(id) ON DELETE CASCADE,
    filename      TEXT        NOT NULL,
    content_type  TEXT        NOT NULL,
    file_size_bytes BIGINT   NOT NULL,
    storage_path  TEXT        NOT NULL,
    is_inline     BOOLEAN     NOT NULL DEFAULT FALSE,
    content_id    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_email_attachments_template ON email_template_attachments (template_id);
