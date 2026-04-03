-- Rollback migration 036: drop all campaign lifecycle tables.

DROP TRIGGER IF EXISTS trg_campaign_config_templates_updated_at ON campaign_config_templates;
DROP TABLE IF EXISTS campaign_config_templates;

DROP TRIGGER IF EXISTS trg_campaign_emails_updated_at ON campaign_emails;
DROP TABLE IF EXISTS campaign_emails;

DROP TABLE IF EXISTS campaign_build_logs;
DROP TABLE IF EXISTS campaign_state_transitions;
DROP TABLE IF EXISTS campaign_target_variant_assignments;
DROP TABLE IF EXISTS campaign_targets_snapshot;
DROP TABLE IF EXISTS campaign_send_windows;
DROP TABLE IF EXISTS campaign_template_variants;

-- Remove FKs added to pre-existing tables.
ALTER TABLE blocklist_overrides DROP CONSTRAINT IF EXISTS fk_blocklist_overrides_campaign;
ALTER TABLE campaign_canary_targets DROP CONSTRAINT IF EXISTS fk_campaign_canary_targets_campaign;
ALTER TABLE campaign_target_groups DROP CONSTRAINT IF EXISTS fk_campaign_target_groups_campaign;
ALTER TABLE campaign_target_events DROP CONSTRAINT IF EXISTS fk_campaign_target_events_campaign;
ALTER TABLE campaign_targets DROP CONSTRAINT IF EXISTS fk_campaign_targets_campaign;

DROP TRIGGER IF EXISTS trg_campaigns_updated_at ON campaigns;
DROP TABLE IF EXISTS campaigns;
