DROP INDEX IF EXISTS idx_campaign_emails_tracking_token;
ALTER TABLE campaign_emails DROP COLUMN IF EXISTS tracking_token;

DROP INDEX IF EXISTS idx_campaign_targets_tracking_token;
ALTER TABLE campaign_targets DROP COLUMN IF EXISTS tracking_token;
