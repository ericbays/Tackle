-- Add tracking_token column to campaign_targets for token resolution.
ALTER TABLE campaign_targets ADD COLUMN IF NOT EXISTS tracking_token VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_campaign_targets_tracking_token
    ON campaign_targets(tracking_token) WHERE tracking_token IS NOT NULL;

-- Add tracking_token column to campaign_emails for per-email token reference.
ALTER TABLE campaign_emails ADD COLUMN IF NOT EXISTS tracking_token VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_campaign_emails_tracking_token
    ON campaign_emails(tracking_token) WHERE tracking_token IS NOT NULL;
