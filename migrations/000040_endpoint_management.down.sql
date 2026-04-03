-- 000040 down: Remove endpoint management tables.

ALTER TABLE campaigns DROP COLUMN IF EXISTS auto_terminate_endpoint;

DROP TABLE IF EXISTS phishing_reports;
DROP TABLE IF EXISTS endpoint_tls_certificates;
DROP TABLE IF EXISTS endpoint_request_logs;
DROP TABLE IF EXISTS endpoint_heartbeats;
