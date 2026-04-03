-- 000040: Endpoint management tables for heartbeats, request logs, TLS certs, and phishing reports.

-- Heartbeat data (time-series resource usage from endpoints).
CREATE TABLE endpoint_heartbeats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES phishing_endpoints(id),
    campaign_id UUID REFERENCES campaigns(id),
    uptime_seconds BIGINT NOT NULL DEFAULT 0,
    cpu_usage_pct DOUBLE PRECISION NOT NULL DEFAULT 0,
    memory_used_bytes BIGINT NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_used_bytes BIGINT NOT NULL DEFAULT 0,
    disk_total_bytes BIGINT NOT NULL DEFAULT 0,
    active_connections INT NOT NULL DEFAULT 0,
    total_requests BIGINT NOT NULL DEFAULT 0,
    total_emails BIGINT NOT NULL DEFAULT 0,
    log_buffer_depth INT NOT NULL DEFAULT 0,
    reported_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_endpoint_heartbeats_endpoint_id ON endpoint_heartbeats(endpoint_id);
CREATE INDEX idx_endpoint_heartbeats_reported_at ON endpoint_heartbeats(endpoint_id, reported_at DESC);

-- Request logs from endpoint proxies (batched delivery).
CREATE TABLE endpoint_request_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES phishing_endpoints(id),
    campaign_id UUID REFERENCES campaigns(id),
    source_ip TEXT NOT NULL,
    http_method TEXT NOT NULL,
    request_path TEXT NOT NULL,
    query_string TEXT,
    request_headers JSONB,
    response_status INT NOT NULL,
    response_size_bytes BIGINT NOT NULL DEFAULT 0,
    response_time_ms INT NOT NULL DEFAULT 0,
    tls_version TEXT,
    logged_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_endpoint_request_logs_endpoint_id ON endpoint_request_logs(endpoint_id);
CREATE INDEX idx_endpoint_request_logs_campaign_id ON endpoint_request_logs(campaign_id);
CREATE INDEX idx_endpoint_request_logs_logged_at ON endpoint_request_logs(endpoint_id, logged_at DESC);
CREATE INDEX idx_endpoint_request_logs_source_ip ON endpoint_request_logs(source_ip);

-- Uploaded TLS certificates for manual certificate management.
CREATE TABLE endpoint_tls_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES phishing_endpoints(id),
    domain TEXT NOT NULL,
    cert_pem_encrypted BYTEA NOT NULL,
    key_pem_encrypted BYTEA NOT NULL,
    issuer TEXT,
    not_before TIMESTAMPTZ NOT NULL,
    not_after TIMESTAMPTZ NOT NULL,
    fingerprint_sha256 TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    uploaded_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    replaced_at TIMESTAMPTZ
);

CREATE INDEX idx_endpoint_tls_certs_endpoint_id ON endpoint_tls_certificates(endpoint_id);

-- Phishing report tracking (webhook + manual).
CREATE TABLE phishing_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID REFERENCES campaigns(id),
    target_id UUID,
    reporter_email TEXT NOT NULL,
    message_id TEXT,
    subject_line TEXT,
    matched BOOLEAN NOT NULL DEFAULT false,
    source TEXT NOT NULL CHECK (source IN ('webhook', 'manual')),
    reported_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_phishing_reports_campaign_id ON phishing_reports(campaign_id);
CREATE INDEX idx_phishing_reports_message_id ON phishing_reports(message_id);

-- Add auto_terminate_on_completion flag to campaigns.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS auto_terminate_endpoint BOOLEAN NOT NULL DEFAULT true;
