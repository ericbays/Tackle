-- Rollback 013.
DROP TRIGGER IF EXISTS trg_notification_smtp_config_updated_at ON notification_smtp_config;
DROP TABLE IF EXISTS notification_smtp_config;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TRIGGER IF EXISTS trg_webhook_endpoints_updated_at ON webhook_endpoints;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS notification_preferences;
DROP INDEX IF EXISTS idx_notifications_expires_at;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP INDEX IF EXISTS idx_notifications_user_id;
DROP TABLE IF EXISTS notifications;
