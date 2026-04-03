-- Reverse migration 042: Email delivery pipeline support.

DROP TABLE IF EXISTS campaign_smtp_send_counts;
DROP TABLE IF EXISTS email_delivery_events;

DROP INDEX IF EXISTS idx_ce_retry;

ALTER TABLE campaign_emails DROP COLUMN IF EXISTS variant_label;
ALTER TABLE campaign_emails DROP COLUMN IF EXISTS next_retry_at;
