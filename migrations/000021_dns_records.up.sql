-- Migration 000021: DNS record cache, propagation checks, and DKIM keys.

-- dns_records caches DNS records fetched from provider APIs.
-- Records here mirror what is live on the provider; mutations go through the service.
CREATE TABLE IF NOT EXISTS dns_records (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id       UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    provider_record_id      TEXT,                -- provider-assigned record ID (may be null for type/name-keyed providers)
    record_type             TEXT NOT NULL,       -- A, AAAA, CNAME, MX, TXT, NS, SOA
    record_name             TEXT NOT NULL,       -- relative record name (e.g. "@", "www", "mail")
    record_value            TEXT NOT NULL,
    ttl                     INTEGER NOT NULL DEFAULT 300,
    priority                INTEGER NOT NULL DEFAULT 0,
    synced_at               TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS dns_records_domain_profile_id_idx ON dns_records(domain_profile_id);
CREATE UNIQUE INDEX IF NOT EXISTS dns_records_type_name_idx ON dns_records(domain_profile_id, record_type, record_name);

-- dns_propagation_checks records the result of propagation verification after DNS changes.
CREATE TABLE IF NOT EXISTS dns_propagation_checks (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id   UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    record_type         TEXT NOT NULL,
    record_name         TEXT NOT NULL,
    expected_value      TEXT NOT NULL,
    overall_status      TEXT NOT NULL DEFAULT 'not_propagated', -- propagated | partial | not_propagated
    results_json        JSONB NOT NULL DEFAULT '[]',            -- per-resolver results
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS dns_propagation_checks_domain_profile_id_idx ON dns_propagation_checks(domain_profile_id);
CREATE INDEX IF NOT EXISTS dns_propagation_checks_checked_at_idx ON dns_propagation_checks(checked_at DESC);

-- dkim_keys stores generated DKIM key pairs per domain.
CREATE TABLE IF NOT EXISTS dkim_keys (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_profile_id       UUID NOT NULL REFERENCES domain_profiles(id) ON DELETE CASCADE,
    selector                TEXT NOT NULL,
    algorithm               TEXT NOT NULL DEFAULT 'rsa-sha256',  -- rsa-sha256 | ed25519-sha256
    key_size                INTEGER,                              -- RSA key size in bits; NULL for Ed25519
    private_key_encrypted   BYTEA NOT NULL,                      -- AES-256-GCM encrypted private key PEM
    public_key              TEXT NOT NULL,                       -- base64-encoded public key (p= field value)
    rotated_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (domain_profile_id, selector)
);

CREATE INDEX IF NOT EXISTS dkim_keys_domain_profile_id_idx ON dkim_keys(domain_profile_id);
