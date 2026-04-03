-- Migration 026 rollback: drop instance_templates tables.

DROP TABLE IF EXISTS instance_template_versions;
DROP TABLE IF EXISTS instance_templates;
