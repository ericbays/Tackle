-- Migration 028: campaign_smtp_profiles association table.

CREATE TABLE campaign_smtp_profiles (
    id                    UUID          NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    campaign_id           UUID          NOT NULL,
    smtp_profile_id       UUID          NOT NULL REFERENCES smtp_profiles(id) ON DELETE RESTRICT,
    priority              INTEGER       NOT NULL DEFAULT 0,
    weight                INTEGER       NOT NULL DEFAULT 0,
    from_address_override VARCHAR(255),
    from_name_override    VARCHAR(255),
    reply_to_override     VARCHAR(255),
    segment_filter        JSONB,
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT uq_campaign_smtp_profiles UNIQUE (campaign_id, smtp_profile_id)
);

CREATE INDEX idx_campaign_smtp_profiles_campaign_id ON campaign_smtp_profiles (campaign_id);
