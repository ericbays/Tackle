-- Migration 046: Add missing columns, constraints, and triggers.

-- Add timestamps to permissions table
ALTER TABLE permissions ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE permissions ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE TRIGGER trg_permissions_updated_at BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Add updated_at to sessions
ALTER TABLE sessions ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE TRIGGER trg_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Add variables JSONB to email_templates
ALTER TABLE email_templates ADD COLUMN variables JSONB;

-- Add managed_by_system to dns_records
ALTER TABLE dns_records ADD COLUMN managed_by_system BOOLEAN NOT NULL DEFAULT FALSE;

-- Add CHECK constraint on dns_records.record_type (if not already present)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.check_constraints
        WHERE constraint_name = 'chk_dns_records_record_type'
    ) THEN
        ALTER TABLE dns_records ADD CONSTRAINT chk_dns_records_record_type
            CHECK (record_type IN ('A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'SPF', 'DKIM', 'DMARC'));
    END IF;
END $$;

-- Add updated_at trigger to dns_records if missing
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.triggers
        WHERE trigger_name = 'trg_dns_records_updated_at'
    ) THEN
        CREATE TRIGGER trg_dns_records_updated_at BEFORE UPDATE ON dns_records
            FOR EACH ROW EXECUTE FUNCTION set_updated_at();
    END IF;
END $$;

-- Add auto_renew to domain_profiles
ALTER TABLE domain_profiles ADD COLUMN auto_renew BOOLEAN NOT NULL DEFAULT FALSE;

-- Add instance_type and tls_cert_info to phishing_endpoints
ALTER TABLE phishing_endpoints ADD COLUMN instance_type VARCHAR(128);
ALTER TABLE phishing_endpoints ADD COLUMN tls_cert_info JSONB;

-- Partial index on public_ip WHERE NOT NULL
CREATE INDEX idx_phishing_endpoints_public_ip ON phishing_endpoints (public_ip) WHERE public_ip IS NOT NULL;

-- FK on campaign_smtp_profiles.campaign_id
ALTER TABLE campaign_smtp_profiles
    ADD CONSTRAINT fk_campaign_smtp_profiles_campaign
    FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE CASCADE;

-- FK on ai_proposals.campaign_id (if column exists)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ai_proposals' AND column_name = 'campaign_id'
    ) THEN
        ALTER TABLE ai_proposals
            ADD CONSTRAINT fk_ai_proposals_campaign
            FOREIGN KEY (campaign_id) REFERENCES campaigns(id) ON DELETE SET NULL;
    END IF;
END $$;

-- Partial index on ai_proposals.campaign_id
CREATE INDEX idx_ai_proposals_campaign_id ON ai_proposals (campaign_id) WHERE campaign_id IS NOT NULL;

-- Add cloud_credential_id FK to phishing_endpoints
ALTER TABLE phishing_endpoints ADD COLUMN cloud_credential_id UUID;
ALTER TABLE phishing_endpoints
    ADD CONSTRAINT fk_phishing_endpoints_cloud_credential
    FOREIGN KEY (cloud_credential_id) REFERENCES cloud_credentials(id) ON DELETE RESTRICT;
CREATE INDEX idx_phishing_endpoints_cloud_credential ON phishing_endpoints (cloud_credential_id) WHERE cloud_credential_id IS NOT NULL;
