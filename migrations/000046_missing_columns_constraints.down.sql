-- Reverse migration 046: remove added columns, constraints, and triggers.

DROP INDEX IF EXISTS idx_phishing_endpoints_cloud_credential;
ALTER TABLE phishing_endpoints DROP CONSTRAINT IF EXISTS fk_phishing_endpoints_cloud_credential;
ALTER TABLE phishing_endpoints DROP COLUMN IF EXISTS cloud_credential_id;

DROP INDEX IF EXISTS idx_ai_proposals_campaign_id;
ALTER TABLE ai_proposals DROP CONSTRAINT IF EXISTS fk_ai_proposals_campaign;

ALTER TABLE campaign_smtp_profiles DROP CONSTRAINT IF EXISTS fk_campaign_smtp_profiles_campaign;

DROP INDEX IF EXISTS idx_phishing_endpoints_public_ip;
ALTER TABLE phishing_endpoints DROP COLUMN IF EXISTS tls_cert_info;
ALTER TABLE phishing_endpoints DROP COLUMN IF EXISTS instance_type;

ALTER TABLE domain_profiles DROP COLUMN IF EXISTS auto_renew;

DROP TRIGGER IF EXISTS trg_dns_records_updated_at ON dns_records;
ALTER TABLE dns_records DROP CONSTRAINT IF EXISTS chk_dns_records_record_type;
ALTER TABLE dns_records DROP COLUMN IF EXISTS managed_by_system;

ALTER TABLE email_templates DROP COLUMN IF EXISTS variables;

DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
ALTER TABLE sessions DROP COLUMN IF EXISTS updated_at;

DROP TRIGGER IF EXISTS trg_permissions_updated_at ON permissions;
ALTER TABLE permissions DROP COLUMN IF EXISTS updated_at;
ALTER TABLE permissions DROP COLUMN IF EXISTS created_at;
