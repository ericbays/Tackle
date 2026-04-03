-- Migration 052: Add denormalized timestamp columns to campaign_targets and update status CHECK.

ALTER TABLE campaign_targets ADD COLUMN sent_at TIMESTAMPTZ;
ALTER TABLE campaign_targets ADD COLUMN opened_at TIMESTAMPTZ;
ALTER TABLE campaign_targets ADD COLUMN clicked_at TIMESTAMPTZ;
ALTER TABLE campaign_targets ADD COLUMN submitted_at TIMESTAMPTZ;
ALTER TABLE campaign_targets ADD COLUMN reported_at TIMESTAMPTZ;

-- Update CHECK constraint to include 'reported' status
ALTER TABLE campaign_targets DROP CONSTRAINT IF EXISTS campaign_targets_status_check;
ALTER TABLE campaign_targets ADD CONSTRAINT campaign_targets_status_check
    CHECK (status IN ('pending', 'email_sent', 'email_opened', 'link_clicked', 'credential_submitted', 'reported'));
