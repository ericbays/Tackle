-- Migration 013: notification tables.

-- notifications: per-user in-app notifications.
CREATE TABLE notifications (
    id            UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category      TEXT        NOT NULL,
    severity      TEXT        NOT NULL DEFAULT 'info'
                      CHECK (severity IN ('info', 'warning', 'critical')),
    title         TEXT        NOT NULL,
    body          TEXT        NOT NULL,
    resource_type TEXT,
    resource_id   UUID,
    action_url    TEXT,
    is_read       BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_id    ON notifications (user_id, created_at DESC);
CREATE INDEX idx_notifications_user_unread ON notifications (user_id) WHERE is_read = FALSE;
CREATE INDEX idx_notifications_expires_at  ON notifications (expires_at)
    WHERE expires_at IS NOT NULL;

-- notification_preferences: per-user per-category delivery preferences.
CREATE TABLE notification_preferences (
    id              UUID    NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id         UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category        TEXT    NOT NULL,
    email_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    email_mode      TEXT    NOT NULL DEFAULT 'digest'
                        CHECK (email_mode IN ('immediate', 'digest')),
    digest_interval TEXT    NOT NULL DEFAULT 'daily'
                        CHECK (digest_interval IN ('hourly', 'daily', 'weekly')),
    UNIQUE (user_id, category)
);

-- webhook_endpoints: outbound webhook delivery targets.
CREATE TABLE webhook_endpoints (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    name        TEXT        NOT NULL,
    url         TEXT        NOT NULL,
    auth_type   TEXT        NOT NULL
                    CHECK (auth_type IN ('none', 'hmac', 'bearer', 'basic')),
    auth_config BYTEA,
    categories  TEXT[],
    is_enabled  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by  UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_webhook_endpoints_updated_at
    BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- webhook_deliveries: delivery attempt records per notification.
CREATE TABLE webhook_deliveries (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    webhook_id      UUID        NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL
                        CHECK (status IN ('success', 'failed', 'pending')),
    response_code   INTEGER,
    response_body   TEXT,
    attempted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    retry_count     INTEGER     NOT NULL DEFAULT 0
);

-- notification_smtp_config: SMTP settings for notification email delivery.
CREATE TABLE notification_smtp_config (
    id           UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    host         TEXT        NOT NULL,
    port         INTEGER     NOT NULL,
    auth_type    TEXT        NOT NULL DEFAULT 'plain',
    username     TEXT,
    password     BYTEA,
    tls_mode     TEXT        NOT NULL DEFAULT 'starttls',
    from_address TEXT        NOT NULL,
    from_name    TEXT        NOT NULL DEFAULT 'Tackle',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_notification_smtp_config_updated_at
    BEFORE UPDATE ON notification_smtp_config
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
