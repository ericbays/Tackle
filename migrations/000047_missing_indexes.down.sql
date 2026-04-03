-- Reverse migration 047: drop added indexes.

DROP INDEX IF EXISTS idx_target_groups_created_by;
DROP INDEX IF EXISTS idx_phishing_endpoints_active;
DROP INDEX IF EXISTS idx_cte_campaign_event_created;
DROP INDEX IF EXISTS idx_campaign_targets_campaign_status;
