-- Migration 047: Add missing composite and partial indexes.

-- Composite index on campaign_targets for status filtering by campaign
CREATE INDEX idx_campaign_targets_campaign_status ON campaign_targets (campaign_id, status);

-- Composite index on campaign_target_events for metrics aggregation
CREATE INDEX idx_cte_campaign_event_created ON campaign_target_events (campaign_id, event_type, created_at);

-- Partial index on phishing_endpoints for active endpoints
CREATE INDEX idx_phishing_endpoints_active ON phishing_endpoints (state) WHERE state NOT IN ('terminated', 'error');

-- Index on target_groups.created_by
CREATE INDEX idx_target_groups_created_by ON target_groups (created_by);
