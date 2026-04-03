-- 000041 down: Drop credential capture tables and types.

ALTER TABLE landing_page_projects DROP COLUMN IF EXISTS session_capture_scope;
ALTER TABLE landing_page_projects DROP COLUMN IF EXISTS session_capture_enabled;
ALTER TABLE landing_page_projects DROP COLUMN IF EXISTS post_capture_config;
ALTER TABLE landing_page_projects DROP COLUMN IF EXISTS post_capture_action;

DROP TABLE IF EXISTS field_categorization_rules;
DROP TABLE IF EXISTS session_captures;
DROP TABLE IF EXISTS capture_fields;
DROP TABLE IF EXISTS capture_events;

DROP TYPE IF EXISTS post_capture_action;
DROP TYPE IF EXISTS session_data_type;
DROP TYPE IF EXISTS field_category;
