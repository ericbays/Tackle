-- Migration 012: archive_metadata table.
-- Tracks partition archival state for audit log lifecycle management (REQ-LOG-018).
-- Empty in v1; schema must be present.

CREATE TABLE archive_metadata (
    id               UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    partition_name   TEXT        NOT NULL UNIQUE,
    time_range_start TIMESTAMPTZ NOT NULL,
    time_range_end   TIMESTAMPTZ NOT NULL,
    archived_at      TIMESTAMPTZ,
    s3_bucket        TEXT,
    s3_key           TEXT,
    row_count        BIGINT,
    status           TEXT        NOT NULL DEFAULT 'active'
                         CHECK (status IN ('active', 'archiving', 'archived', 'error')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
