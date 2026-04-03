-- Migration 000022: Email authentication status tracking.

-- domain_email_auth_status stores the current SPF/DKIM/DMARC status per domain.
CREATE TABLE IF NOT EXISTS domain_email_auth_status (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id   UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    spf_status          TEXT NOT NULL DEFAULT 'missing',     -- configured | misconfigured | missing
    dkim_status         TEXT NOT NULL DEFAULT 'missing',     -- configured | misconfigured | missing
    dmarc_status        TEXT NOT NULL DEFAULT 'missing',     -- configured | misconfigured | missing
    last_checked_at     TIMESTAMPTZ,
    details_json        JSONB NOT NULL DEFAULT '{}',         -- per-mechanism detail and diffs
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (domain_profile_id)
);

CREATE INDEX IF NOT EXISTS domain_email_auth_status_domain_profile_id_idx ON domain_email_auth_status(domain_profile_id);
