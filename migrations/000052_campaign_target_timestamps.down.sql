-- Reverse migration 052: remove timestamp columns and restore original CHECK.

ALTER TABLE campaign_targets DROP CONSTRAINT IF EXISTS campaign_targets_status_check;
ALTER TABLE campaign_targets ADD CONSTRAINT campaign_targets_status_check
    CHECK (status IN ('pending', 'email_sent', 'email_opened', 'link_clicked', 'credential_submitted'));

ALTER TABLE campaign_targets DROP COLUMN IF EXISTS reported_at;
ALTER TABLE campaign_targets DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE campaign_targets DROP COLUMN IF EXISTS clicked_at;
ALTER TABLE campaign_targets DROP COLUMN IF EXISTS opened_at;
ALTER TABLE campaign_targets DROP COLUMN IF EXISTS sent_at;
