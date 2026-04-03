-- Migration 011: audit_logs partitioned table with immutability trigger.
-- Partitioned by range on timestamp (REQ-LOG-018).
-- Application DB user (tackle_app) is restricted to INSERT and SELECT only.

CREATE TABLE audit_logs (
    id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    timestamp      TIMESTAMPTZ NOT NULL DEFAULT now(),
    category       TEXT        NOT NULL
                       CHECK (category IN ('user_activity', 'email_event', 'infrastructure', 'request', 'system')),
    severity       TEXT        NOT NULL DEFAULT 'info'
                       CHECK (severity IN ('debug', 'info', 'warning', 'error', 'critical')),
    actor_type     TEXT        NOT NULL
                       CHECK (actor_type IN ('user', 'system', 'endpoint', 'external')),
    actor_id       UUID,
    actor_label    TEXT,
    action         TEXT        NOT NULL,
    resource_type  TEXT,
    resource_id    UUID,
    details        JSONB,
    correlation_id UUID,
    source_ip      INET,
    session_id     UUID,
    campaign_id    UUID,
    checksum       TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create the immutability trigger on the parent table.
-- PostgreSQL fires row-level triggers on each partition automatically.
CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION reject_modification();

-- Create initial monthly partitions: current month + next 3 months.
-- Current month baseline is 2026-03 per project date; SQL uses date_trunc for portability.
CREATE TABLE audit_logs_2026_03 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE audit_logs_2026_04 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

CREATE TABLE audit_logs_2026_05 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE TABLE audit_logs_2026_06 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- Indexes on the parent table propagate to each partition.
CREATE INDEX idx_audit_logs_timestamp       ON audit_logs (timestamp DESC);
CREATE INDEX idx_audit_logs_category        ON audit_logs (category, timestamp DESC);
CREATE INDEX idx_audit_logs_actor_id        ON audit_logs (actor_id, timestamp DESC)
    WHERE actor_id IS NOT NULL;
CREATE INDEX idx_audit_logs_resource        ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_logs_correlation_id  ON audit_logs (correlation_id)
    WHERE correlation_id IS NOT NULL;
CREATE INDEX idx_audit_logs_severity        ON audit_logs (severity, timestamp DESC)
    WHERE severity IN ('warning', 'error', 'critical');
CREATE INDEX idx_audit_logs_action          ON audit_logs (action, timestamp DESC);
CREATE INDEX idx_audit_logs_campaign_id     ON audit_logs (campaign_id, timestamp DESC)
    WHERE campaign_id IS NOT NULL;
CREATE INDEX idx_audit_logs_details_gin     ON audit_logs USING GIN (details);

-- Restrict the application database user to INSERT and SELECT only (REQ-LOG-020).
-- The tackle_app role is created here if it does not exist yet.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'tackle_app') THEN
        CREATE ROLE tackle_app;
    END IF;
END
$$;

REVOKE UPDATE, DELETE ON audit_logs FROM tackle_app;
