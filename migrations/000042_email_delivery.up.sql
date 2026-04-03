-- Migration 042: Email delivery pipeline support.
-- Adds email_delivery_events table for delivery lifecycle tracking,
-- retry scheduling column on campaign_emails, and per-profile send count tracking.

-- Add retry scheduling column to campaign_emails.
ALTER TABLE campaign_emails ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;

-- Add variant_label for quick access without joining variant tables.
ALTER TABLE campaign_emails ADD COLUMN IF NOT EXISTS variant_label VARCHAR(100);

-- Index for retry scheduling: find emails ready for retry.
CREATE INDEX IF NOT EXISTS idx_ce_retry
    ON campaign_emails (campaign_id, next_retry_at)
    WHERE next_retry_at IS NOT NULL AND status IN ('deferred', 'failed');

-- Email delivery events: full lifecycle audit trail per email.
CREATE TABLE email_delivery_events (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    email_id    UUID        NOT NULL REFERENCES campaign_emails(id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL
                CHECK (event_type IN ('queued', 'sending', 'sent', 'delivered', 'deferred', 'bounced', 'failed', 'cancelled', 'retry')),
    event_data  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ede_email_id ON email_delivery_events (email_id);
CREATE INDEX idx_ede_type ON email_delivery_events (email_id, event_type);

-- Per-SMTP-profile send counts per campaign for round-robin tracking.
CREATE TABLE campaign_smtp_send_counts (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id     UUID        NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    smtp_profile_id UUID        NOT NULL,
    send_count      INTEGER     NOT NULL DEFAULT 0,
    error_count     INTEGER     NOT NULL DEFAULT 0,
    last_sent_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_campaign_smtp_send_count UNIQUE (campaign_id, smtp_profile_id)
);

CREATE TRIGGER trg_campaign_smtp_send_counts_updated_at
    BEFORE UPDATE ON campaign_smtp_send_counts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
