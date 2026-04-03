-- ============================================================
-- Tackle Seed Data
-- Run: docker exec -i tackle-postgres-1 psql -U tackle_dev -d tackle_dev < scripts/seed.sql
-- ============================================================

-- Admin user ID (created by setup): 2b3b120b-e324-463b-9ae5-70fe5138e368
-- All seeded user passwords: Password123!
-- bcrypt hash for Password123!
-- $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy

-- ---- Clean slate: truncate all seeded data ----
TRUNCATE notification_preferences, notifications, sessions, api_keys CASCADE;
TRUNCATE campaign_send_windows, campaign_template_variants, campaign_smtp_profiles,
         campaign_target_groups, campaign_targets, campaign_state_transitions,
         campaign_approvals, campaign_build_logs, campaign_emails,
         campaign_canary_targets, campaign_targets_snapshot,
         campaign_target_variant_assignments, campaign_smtp_send_counts,
         campaign_shares, campaign_config_templates, approval_comments CASCADE;
TRUNCATE campaigns CASCADE;
TRUNCATE capture_events, capture_fields, session_captures, field_categorization_rules CASCADE;
TRUNCATE target_group_members, target_groups CASCADE;
TRUNCATE blocklist_entries, blocklist_overrides CASCADE;
TRUNCATE targets CASCADE;
TRUNCATE email_template_attachments, email_template_versions, email_templates CASCADE;
TRUNCATE landing_page_builds, landing_page_templates, landing_page_projects CASCADE;
TRUNCATE smtp_profiles CASCADE;
TRUNCATE phishing_endpoints, endpoint_heartbeats, endpoint_request_logs,
         endpoint_state_transitions, endpoint_ssh_keys, endpoint_tls_certificates CASCADE;
TRUNCATE domain_profiles, domain_provider_connections, dns_records,
         domain_health_checks, domain_categorizations, domain_email_auth_status CASCADE;
TRUNCATE cloud_credentials, instance_templates, instance_template_versions CASCADE;
TRUNCATE audit_logs CASCADE;
TRUNCATE webhook_endpoints, webhook_deliveries CASCADE;
TRUNCATE alert_rules CASCADE;
TRUNCATE user_roles CASCADE;
TRUNCATE password_history CASCADE;
DELETE FROM users;

-- ---- Admin user (password: "admin") ----
INSERT INTO users (id, email, username, password_hash, display_name, auth_provider, status, is_initial_admin) VALUES
  ('2b3b120b-e324-463b-9ae5-70fe5138e368', 'admin@tackle.local', 'admin', '$2a$12$1nfy1UGL9z8Svqmezlem6.V/deMSRTbQ0MIwtbp/mIfenl9JGRCjm', 'Admin', 'local', 'active', true);
INSERT INTO user_roles (user_id, role_id) VALUES
  ('2b3b120b-e324-463b-9ae5-70fe5138e368', (SELECT id FROM roles WHERE name='admin'));

-- ---- Additional Users ----
INSERT INTO users (id, email, username, password_hash, display_name, auth_provider, status) VALUES
  ('a1111111-1111-1111-1111-111111111111', 'sarah.chen@acme.corp', 'sarah.chen', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Sarah Chen', 'local', 'active'),
  ('a2222222-2222-2222-2222-222222222222', 'mike.ross@acme.corp', 'mike.ross', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Mike Ross', 'local', 'active'),
  ('a3333333-3333-3333-3333-333333333333', 'jen.martinez@acme.corp', 'jen.martinez', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Jen Martinez', 'local', 'active'),
  ('a4444444-4444-4444-4444-444444444444', 'tom.baker@acme.corp', 'tom.baker', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 'Tom Baker', 'local', 'active')
ON CONFLICT DO NOTHING;

-- Assign roles: sarah=operator, mike=operator, jen=engineer, tom=defender
INSERT INTO user_roles (user_id, role_id) VALUES
  ('a1111111-1111-1111-1111-111111111111', (SELECT id FROM roles WHERE name='operator')),
  ('a2222222-2222-2222-2222-222222222222', (SELECT id FROM roles WHERE name='operator')),
  ('a3333333-3333-3333-3333-333333333333', (SELECT id FROM roles WHERE name='engineer')),
  ('a4444444-4444-4444-4444-444444444444', (SELECT id FROM roles WHERE name='defender'))
ON CONFLICT DO NOTHING;

-- ---- Targets (25 employees across departments) ----
INSERT INTO targets (id, email, first_name, last_name, department, title, created_by) VALUES
  ('b0000001-0001-0001-0001-000000000001', 'john.smith@example.com', 'John', 'Smith', 'Engineering', 'Software Engineer', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000002', 'jane.doe@example.com', 'Jane', 'Doe', 'Engineering', 'Senior Engineer', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000003', 'bob.wilson@example.com', 'Bob', 'Wilson', 'Engineering', 'Tech Lead', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000004', 'alice.jones@example.com', 'Alice', 'Jones', 'Finance', 'Controller', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000005', 'charlie.brown@example.com', 'Charlie', 'Brown', 'Finance', 'Accountant', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000006', 'diana.prince@example.com', 'Diana', 'Prince', 'HR', 'HR Director', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000007', 'eric.taylor@example.com', 'Eric', 'Taylor', 'HR', 'Recruiter', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000008', 'fiona.green@example.com', 'Fiona', 'Green', 'Marketing', 'CMO', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000009', 'george.harris@example.com', 'George', 'Harris', 'Marketing', 'Content Manager', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000010', 'hannah.clark@example.com', 'Hannah', 'Clark', 'Marketing', 'Social Media Lead', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000011', 'ian.wright@example.com', 'Ian', 'Wright', 'Sales', 'VP Sales', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000012', 'julia.roberts@example.com', 'Julia', 'Roberts', 'Sales', 'Account Executive', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000013', 'kevin.lee@example.com', 'Kevin', 'Lee', 'Sales', 'SDR', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000014', 'lisa.nguyen@example.com', 'Lisa', 'Nguyen', 'Engineering', 'DevOps Engineer', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000015', 'mark.johnson@example.com', 'Mark', 'Johnson', 'IT', 'Sysadmin', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000016', 'nancy.white@example.com', 'Nancy', 'White', 'IT', 'Help Desk', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000017', 'oscar.martinez@example.com', 'Oscar', 'Martinez', 'Legal', 'General Counsel', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000018', 'patricia.davis@example.com', 'Patricia', 'Davis', 'Legal', 'Compliance Officer', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000019', 'robert.miller@example.com', 'Robert', 'Miller', 'Executive', 'CEO', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000020', 'susan.anderson@example.com', 'Susan', 'Anderson', 'Executive', 'CFO', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000021', 'tim.moore@example.com', 'Tim', 'Moore', 'Engineering', 'QA Lead', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000022', 'uma.patel@example.com', 'Uma', 'Patel', 'Engineering', 'Data Engineer', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000023', 'victor.chen@example.com', 'Victor', 'Chen', 'Finance', 'Financial Analyst', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000024', 'wendy.kim@example.com', 'Wendy', 'Kim', 'HR', 'Benefits Coordinator', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('b0000001-0001-0001-0001-000000000025', 'xavier.lopez@example.com', 'Xavier', 'Lopez', 'IT', 'Security Analyst', '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- ---- Target Groups ----
INSERT INTO target_groups (id, name, description, created_by) VALUES
  ('c0000001-0001-0001-0001-000000000001', 'All Engineering', 'All engineering department staff', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000002', 'Executive Team', 'C-suite and VP-level executives', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'Finance & Legal', 'Finance and legal departments', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000004', 'New Hires Q1', 'Q1 2026 new hires for baseline assessment', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000005', 'IT & Security', 'IT department and security team', '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- Group members
INSERT INTO target_group_members (group_id, target_id, added_by) VALUES
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000001', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000002', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000003', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000014', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000021', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000001', 'b0000001-0001-0001-0001-000000000022', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000002', 'b0000001-0001-0001-0001-000000000019', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000002', 'b0000001-0001-0001-0001-000000000020', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000002', 'b0000001-0001-0001-0001-000000000011', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000002', 'b0000001-0001-0001-0001-000000000008', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'b0000001-0001-0001-0001-000000000004', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'b0000001-0001-0001-0001-000000000005', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'b0000001-0001-0001-0001-000000000017', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'b0000001-0001-0001-0001-000000000018', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000003', 'b0000001-0001-0001-0001-000000000023', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000005', 'b0000001-0001-0001-0001-000000000015', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000005', 'b0000001-0001-0001-0001-000000000016', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('c0000001-0001-0001-0001-000000000005', 'b0000001-0001-0001-0001-000000000025', '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- ---- Blocklist Entries ----
INSERT INTO blocklist_entries (id, pattern, reason, is_active, added_by) VALUES
  ('d0000001-0001-0001-0001-000000000001', 'ceo@example.com', 'CEO explicitly excluded from all campaigns', true, '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('d0000001-0001-0001-0001-000000000002', '@legal-external.com', 'External legal counsel domain - never target', true, '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('d0000001-0001-0001-0001-000000000003', 'contractor-*@example.com', 'All contractor accounts excluded', true, '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- ---- Email Templates ----
INSERT INTO email_templates (id, name, description, subject, html_body, text_body, category, tags, created_by) VALUES
  ('e0000001-0001-0001-0001-000000000001', 'IT Password Reset', 'Standard IT password reset phish', 'Action Required: Password Expiration Notice', '<html><body><h2>IT Security Notice</h2><p>Dear {{.FirstName}},</p><p>Your network password will expire in 24 hours. Please reset it immediately to maintain access.</p><p><a href="{{.TrackingURL}}">Reset Password Now</a></p><p>IT Security Team</p></body></html>', 'Dear {{.FirstName}}, Your password will expire in 24 hours. Reset at: {{.TrackingURL}}', 'credential_harvest', '{password,it,urgent}', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('e0000001-0001-0001-0001-000000000002', 'HR Benefits Update', 'Open enrollment benefits notification', 'Important: Open Enrollment Deadline Approaching', '<html><body><h2>Benefits Open Enrollment</h2><p>Hi {{.FirstName}},</p><p>Open enrollment for 2026 benefits ends this Friday.</p><p><a href="{{.TrackingURL}}">Review Benefits</a></p><p>HR Department</p></body></html>', 'Hi {{.FirstName}}, Open enrollment ends Friday. Review: {{.TrackingURL}}', 'credential_harvest', '{hr,benefits,deadline}', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('e0000001-0001-0001-0001-000000000003', 'DocuSign Contract', 'Fake DocuSign signature request', 'Please Sign: Updated Employment Agreement', '<html><body><div style="background:#f5f5f5;padding:20px;"><h3>Document Ready for Signature</h3><p>{{.FirstName}} {{.LastName}},</p><p>Your updated employment agreement is ready.</p><p><a href="{{.TrackingURL}}">REVIEW DOCUMENT</a></p></div></body></html>', '{{.FirstName}}, Your agreement is ready: {{.TrackingURL}}', 'credential_harvest', '{docusign,contract,signature}', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('e0000001-0001-0001-0001-000000000004', 'Slack Notification', 'Fake Slack workspace notification', 'New message from your manager in #general', '<html><body><div style="background:#4a154b;padding:30px;"><div style="background:#fff;border-radius:8px;padding:24px;"><h3>New direct message</h3><p>Your manager sent you a message in <strong>#general</strong></p><p><a href="{{.TrackingURL}}">Open in Slack</a></p></div></div></body></html>', 'New Slack message from your manager. Open: {{.TrackingURL}}', 'awareness', '{slack,social,notification}', '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('e0000001-0001-0001-0001-000000000005', 'Microsoft 365 Alert', 'Fake M365 security alert', 'Unusual Sign-in Activity Detected on Your Account', '<html><body><div style="max-width:600px;margin:0 auto;"><div style="background:#0078d4;color:#fff;padding:16px 24px;"><strong>Microsoft</strong></div><div style="padding:24px;border:1px solid #ddd;"><h3>Unusual sign-in activity</h3><p>We detected a sign-in from Moscow, Russia</p><p><a href="{{.TrackingURL}}">Review Activity</a></p></div></div></body></html>', 'Unusual sign-in detected. Review at: {{.TrackingURL}}', 'credential_harvest', '{microsoft,security,urgent}', '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- ---- Landing Page Projects ----

INSERT INTO landing_page_projects (id, name, description, definition_json, created_by, post_capture_action)
VALUES ('f0000001-0001-0001-0001-000000000001', 'Corporate Login Portal', 'Fake SSO login page with credential capture form', $LP1${
    "schema_version": 1,
    "pages": [{
      "page_id": "p1", "name": "Login", "route": "/", "title": "Sign In - Corporate Portal",
      "favicon": "", "meta_tags": [{"name":"viewport","content":"width=device-width, initial-scale=1.0"}],
      "page_styles": ".login-card{max-width:400px;margin:80px auto;padding:40px;background:#fff;border-radius:12px;box-shadow:0 4px 24px rgba(0,0,0,0.08)}",
      "page_js": "",
      "component_tree": [
        {
          "component_id": "hdr1", "type": "container", "properties": {"inline_style":"background:#1a365d;padding:16px 32px;display:flex;align-items:center;justify-content:space-between"},
          "children": [
            {"component_id": "logo1", "type": "heading", "properties": {"content":"ACME Corp","tag":"h1","inline_style":"color:#fff;font-size:20px;margin:0;font-weight:600"}, "children": [], "event_bindings": []},
            {"component_id": "nav1", "type": "text", "properties": {"content":"Employee Portal","inline_style":"color:#a0aec0;font-size:14px"}, "children": [], "event_bindings": []}
          ], "event_bindings": []
        },
        {
          "component_id": "main1", "type": "container", "properties": {"css_class":"login-card"},
          "children": [
            {"component_id": "h2a", "type": "heading", "properties": {"content":"Sign In","tag":"h2","inline_style":"text-align:center;margin-bottom:8px;color:#1a202c"}, "children": [], "event_bindings": []},
            {"component_id": "sub1", "type": "text", "properties": {"content":"Enter your credentials to access the employee portal","inline_style":"text-align:center;color:#718096;font-size:14px;margin-bottom:24px"}, "children": [], "event_bindings": []},
            {
              "component_id": "form1", "type": "form", "properties": {"name":"login","data-capture":"true","data-post-action":"redirect","data-redirect-url":"https://portal.example.com","inline_style":"display:flex;flex-direction:column;gap:16px"},
              "children": [
                {"component_id": "email1", "type": "email_input", "properties": {"name":"email","placeholder":"Email Address","label_text":"Email","required":true,"capture_tag":"email","inline_style":"width:100%;padding:10px 14px;border:1px solid #e2e8f0;border-radius:8px;font-size:14px"}, "children": [], "event_bindings": []},
                {"component_id": "pass1", "type": "password_input", "properties": {"name":"password","placeholder":"Password","label_text":"Password","required":true,"capture_tag":"password","inline_style":"width:100%;padding:10px 14px;border:1px solid #e2e8f0;border-radius:8px;font-size:14px"}, "children": [], "event_bindings": []},
                {"component_id": "hid1", "type": "hidden_field", "properties": {"name":"client_ts","value_source":"dynamic","dynamic_source":"timestamp","capture_tag":"custom"}, "children": [], "event_bindings": []},
                {"component_id": "hid2", "type": "hidden_field", "properties": {"name":"ref","value_source":"dynamic","dynamic_source":"referrer","capture_tag":"custom"}, "children": [], "event_bindings": []},
                {"component_id": "sub_btn1", "type": "submit_button", "properties": {"content":"Sign In","inline_style":"width:100%;padding:12px;background:#3182ce;color:#fff;border:none;border-radius:8px;font-size:16px;font-weight:600;cursor:pointer"}, "children": [], "event_bindings": []}
              ], "event_bindings": []
            },
            {"component_id": "forgot1", "type": "link", "properties": {"content":"Forgot password?","href":"#","inline_style":"display:block;text-align:center;color:#3182ce;font-size:13px;margin-top:12px;text-decoration:none"}, "children": [], "event_bindings": []}
          ], "event_bindings": []
        },
        {
          "component_id": "ftr1", "type": "container", "properties": {"inline_style":"text-align:center;padding:24px;color:#a0aec0;font-size:12px"},
          "children": [
            {"component_id": "ftrtxt", "type": "text", "properties": {"content":"© 2026 ACME Corporation. All rights reserved."}, "children": [], "event_bindings": []}
          ], "event_bindings": []
        }
      ]
    }],
    "global_styles": "body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;background:#f0f2f5;margin:0;padding:0}*{box-sizing:border-box}",
    "global_js": "",
    "theme": {},
    "navigation": []
  }$LP1$, '2b3b120b-e324-463b-9ae5-70fe5138e368', 'redirect')
ON CONFLICT DO NOTHING;

INSERT INTO landing_page_projects (id, name, description, definition_json, created_by, post_capture_action)
VALUES ('f0000001-0001-0001-0001-000000000002', 'Password Reset Page', 'IT help desk password reset with MFA capture', $LP2${
    "schema_version": 1,
    "pages": [{
      "page_id": "p1", "name": "Reset Password", "route": "/", "title": "IT Support - Password Reset",
      "favicon": "", "meta_tags": [{"name":"viewport","content":"width=device-width, initial-scale=1.0"}],
      "page_styles": ".reset-box{max-width:480px;margin:60px auto;padding:36px;background:#fff;border-radius:8px;border:1px solid #e2e8f0}",
      "page_js": "",
      "component_tree": [
        {
          "component_id": "banner1", "type": "container", "properties": {"inline_style":"background:#2d3748;padding:12px 24px;display:flex;align-items:center;gap:12px"},
          "children": [
            {"component_id": "icon1", "type": "text", "properties": {"content":"🔒","inline_style":"font-size:20px"}, "children": [], "event_bindings": []},
            {"component_id": "bannertext", "type": "text", "properties": {"content":"IT Security Portal - Password Management","inline_style":"color:#fff;font-size:15px;font-weight:500"}, "children": [], "event_bindings": []}
          ], "event_bindings": []
        },
        {
          "component_id": "resetbox", "type": "container", "properties": {"css_class":"reset-box"},
          "children": [
            {"component_id": "rh1", "type": "heading", "properties": {"content":"Reset Your Password","tag":"h2","inline_style":"margin-bottom:8px;color:#1a202c;font-size:22px"}, "children": [], "event_bindings": []},
            {"component_id": "rdesc", "type": "text", "properties": {"content":"Your password has expired. Please enter your current credentials and set a new password.","inline_style":"color:#718096;font-size:14px;margin-bottom:20px;line-height:1.5"}, "children": [], "event_bindings": []},
            {
              "component_id": "form2", "type": "form", "properties": {"name":"reset","data-capture":"true","data-post-action":"redirect","data-redirect-url":"https://helpdesk.example.com/success","inline_style":"display:flex;flex-direction:column;gap:14px"},
              "children": [
                {"component_id": "usr2", "type": "text_input", "properties": {"name":"username","placeholder":"Username or Employee ID","label_text":"Username","required":true,"capture_tag":"username","inline_style":"width:100%;padding:10px;border:1px solid #cbd5e0;border-radius:6px"}, "children": [], "event_bindings": []},
                {"component_id": "curpw", "type": "password_input", "properties": {"name":"current_password","placeholder":"Current Password","label_text":"Current Password","required":true,"capture_tag":"password","inline_style":"width:100%;padding:10px;border:1px solid #cbd5e0;border-radius:6px"}, "children": [], "event_bindings": []},
                {"component_id": "newpw", "type": "password_input", "properties": {"name":"new_password","placeholder":"New Password","label_text":"New Password","required":true,"min_length":12,"inline_style":"width:100%;padding:10px;border:1px solid #cbd5e0;border-radius:6px"}, "children": [], "event_bindings": []},
                {"component_id": "mfa2", "type": "text_input", "properties": {"name":"mfa_code","placeholder":"6-digit code from authenticator","label_text":"MFA Code","required":true,"capture_tag":"mfa_token","validation_pattern":"^[0-9]{6}$","inline_style":"width:100%;padding:10px;border:1px solid #cbd5e0;border-radius:6px"}, "children": [], "event_bindings": []},
                {"component_id": "hid3", "type": "hidden_field", "properties": {"name":"ua","value_source":"dynamic","dynamic_source":"user_agent"}, "children": [], "event_bindings": []},
                {"component_id": "sub_btn2", "type": "submit_button", "properties": {"content":"Reset Password","inline_style":"width:100%;padding:12px;background:#e53e3e;color:#fff;border:none;border-radius:6px;font-size:15px;font-weight:600;cursor:pointer"}, "children": [], "event_bindings": []}
              ], "event_bindings": []
            },
            {"component_id": "helplink", "type": "link", "properties": {"content":"Contact IT Support","href":"mailto:support@example.com","inline_style":"display:block;text-align:center;color:#4299e1;font-size:13px;margin-top:16px"}, "children": [], "event_bindings": []}
          ], "event_bindings": []
        }
      ]
    }],
    "global_styles": "body{font-family:Segoe UI,Tahoma,Geneva,Verdana,sans-serif;background:#edf2f7;margin:0}",
    "global_js": "",
    "theme": {},
    "navigation": []
  }$LP2$, '2b3b120b-e324-463b-9ae5-70fe5138e368', 'redirect')
ON CONFLICT DO NOTHING;

INSERT INTO landing_page_projects (id, name, description, definition_json, created_by, post_capture_action)
VALUES ('f0000001-0001-0001-0001-000000000003', 'Document Viewer', 'DocuSign-style document portal with multi-page flow', $LP3${
    "schema_version": 1,
    "pages": [
      {
        "page_id": "p1", "name": "Verify Identity", "route": "/", "title": "DocuSign - Verify Your Identity",
        "favicon": "", "meta_tags": [],
        "page_styles": ".ds-card{max-width:440px;margin:40px auto;padding:32px;background:#fff;border-radius:8px;box-shadow:0 2px 12px rgba(0,0,0,0.1)}",
        "page_js": "",
        "component_tree": [
          {
            "component_id": "dshdr", "type": "container", "properties": {"inline_style":"background:#fff;padding:16px 32px;border-bottom:3px solid #ffc107;display:flex;align-items:center;gap:16px"},
            "children": [
              {"component_id": "dslogo", "type": "heading", "properties": {"content":"DocuSign","tag":"h1","inline_style":"color:#333;font-size:24px;margin:0;font-weight:700"}, "children": [], "event_bindings": []},
              {"component_id": "dssep", "type": "text", "properties": {"content":"|","inline_style":"color:#ccc;font-size:24px"}, "children": [], "event_bindings": []},
              {"component_id": "dstitle", "type": "text", "properties": {"content":"Electronic Signature","inline_style":"color:#666;font-size:14px"}, "children": [], "event_bindings": []}
            ], "event_bindings": []
          },
          {
            "component_id": "dscard", "type": "container", "properties": {"css_class":"ds-card"},
            "children": [
              {"component_id": "dsico", "type": "text", "properties": {"content":"📄","inline_style":"font-size:48px;text-align:center;display:block;margin-bottom:12px"}, "children": [], "event_bindings": []},
              {"component_id": "dsh2", "type": "heading", "properties": {"content":"Verify Your Identity","tag":"h2","inline_style":"text-align:center;font-size:20px;color:#333;margin-bottom:8px"}, "children": [], "event_bindings": []},
              {"component_id": "dsdesc", "type": "text", "properties": {"content":"To view and sign your document, please verify your email address and enter your access code.","inline_style":"text-align:center;color:#666;font-size:14px;line-height:1.5;margin-bottom:20px"}, "children": [], "event_bindings": []},
              {
                "component_id": "dsform", "type": "form", "properties": {"name":"verify","data-capture":"true","data-post-action":"display_page","data-target-page":"/document","inline_style":"display:flex;flex-direction:column;gap:14px"},
                "children": [
                  {"component_id": "dsemail", "type": "email_input", "properties": {"name":"email","placeholder":"you@company.com","label_text":"Email Address","required":true,"capture_tag":"email","inline_style":"width:100%;padding:12px;border:1px solid #ddd;border-radius:4px;font-size:14px"}, "children": [], "event_bindings": []},
                  {"component_id": "dscode", "type": "text_input", "properties": {"name":"access_code","placeholder":"Access Code","label_text":"Access Code","required":true,"capture_tag":"custom","inline_style":"width:100%;padding:12px;border:1px solid #ddd;border-radius:4px;font-size:14px"}, "children": [], "event_bindings": []},
                  {"component_id": "dssubmit", "type": "submit_button", "properties": {"content":"CONTINUE","inline_style":"width:100%;padding:14px;background:#ffc107;color:#333;border:none;border-radius:4px;font-size:16px;font-weight:700;cursor:pointer;letter-spacing:0.5px"}, "children": [], "event_bindings": []}
                ], "event_bindings": []
              }
            ], "event_bindings": []
          }
        ]
      },
      {
        "page_id": "p2", "name": "Document", "route": "/document", "title": "DocuSign - Review Document",
        "favicon": "", "meta_tags": [],
        "page_styles": ".doc-container{max-width:700px;margin:40px auto;background:#fff;border-radius:8px;box-shadow:0 2px 12px rgba(0,0,0,0.1);overflow:hidden}",
        "page_js": "",
        "component_tree": [
          {
            "component_id": "dshdr2", "type": "container", "properties": {"inline_style":"background:#fff;padding:16px 32px;border-bottom:3px solid #ffc107"},
            "children": [
              {"component_id": "dslogo2", "type": "heading", "properties": {"content":"DocuSign","tag":"h1","inline_style":"color:#333;font-size:24px;margin:0"}, "children": [], "event_bindings": []}
            ], "event_bindings": []
          },
          {
            "component_id": "docwrap", "type": "container", "properties": {"css_class":"doc-container"},
            "children": [
              {"component_id": "docbar", "type": "container", "properties": {"inline_style":"background:#2d3748;color:#fff;padding:14px 24px;display:flex;justify-content:space-between;align-items:center"},
                "children": [
                  {"component_id": "docname", "type": "text", "properties": {"content":"Employment_Agreement_2026.pdf","inline_style":"font-size:14px;font-weight:500"}, "children": [], "event_bindings": []},
                  {"component_id": "docstatus", "type": "text", "properties": {"content":"AWAITING YOUR SIGNATURE","inline_style":"font-size:11px;background:#ffc107;color:#333;padding:4px 10px;border-radius:12px;font-weight:600"}, "children": [], "event_bindings": []}
                ], "event_bindings": []
              },
              {"component_id": "docbody", "type": "container", "properties": {"inline_style":"padding:32px 24px"},
                "children": [
                  {"component_id": "doctitle", "type": "heading", "properties": {"content":"Updated Employment Agreement","tag":"h2","inline_style":"font-size:22px;color:#1a202c;margin-bottom:16px"}, "children": [], "event_bindings": []},
                  {"component_id": "doctext1", "type": "text", "properties": {"content":"This agreement outlines the updated terms and conditions of your employment with ACME Corporation, effective April 1, 2026. Please review the document carefully before signing.","inline_style":"color:#4a5568;font-size:14px;line-height:1.7;margin-bottom:16px"}, "children": [], "event_bindings": []},
                  {"component_id": "doctext2", "type": "text", "properties": {"content":"Key changes include: updated compensation package, revised remote work policy, and new intellectual property assignment clause. By signing below, you acknowledge that you have read and agree to these terms.","inline_style":"color:#4a5568;font-size:14px;line-height:1.7;margin-bottom:24px"}, "children": [], "event_bindings": []},
                  {"component_id": "sigbtn", "type": "button", "properties": {"content":"✍️ Click to Sign","inline_style":"padding:14px 32px;background:#ffc107;color:#333;border:2px dashed #e2a500;border-radius:4px;font-size:16px;font-weight:600;cursor:pointer;display:block;margin:0 auto"}, "children": [], "event_bindings": [{"event":"click","handler":"alert('Document signed successfully. You will receive a confirmation email shortly.');"}]}
                ], "event_bindings": []
              }
            ], "event_bindings": []
          }
        ]
      }
    ],
    "global_styles": "body{font-family:Helvetica Neue,Helvetica,Arial,sans-serif;background:#f7f7f7;margin:0}",
    "global_js": "",
    "theme": {},
    "navigation": [{"source_page":"p1","trigger":"form_submit","target_page":"p2","delay_ms":0}]
  }$LP3$, '2b3b120b-e324-463b-9ae5-70fe5138e368', 'display_page')
ON CONFLICT DO NOTHING;

-- ---- SMTP Profiles ----
INSERT INTO smtp_profiles (id, name, description, host, port, from_address, from_name, tls_skip_verify, created_by) VALUES
  ('70000001-0001-0001-0001-000000000001', 'Primary SMTP', 'Main sending profile via internal relay', 'smtp.internal.corp', 587, 'noreply@corporate-services.com', 'Corporate IT', false, '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('70000001-0001-0001-0001-000000000002', 'Backup SMTP', 'Backup relay for high-volume sends', 'smtp-backup.internal.corp', 587, 'notifications@corp-systems.com', 'System Notifications', false, '2b3b120b-e324-463b-9ae5-70fe5138e368'),
  ('70000001-0001-0001-0001-000000000003', 'External Relay', 'External SMTP via SES', 'email-smtp.us-east-1.amazonaws.com', 465, 'alerts@company-alerts.com', 'Security Alerts', true, '2b3b120b-e324-463b-9ae5-70fe5138e368')
ON CONFLICT DO NOTHING;

-- ---- Campaigns (various states) ----
INSERT INTO campaigns (id, name, description, current_state, landing_page_id, throttle_rate, send_order, grace_period_hours, start_date, end_date, created_by, configuration) VALUES
  ('80000001-0001-0001-0001-000000000001', 'Q1 Baseline Assessment', 'Initial phishing susceptibility baseline for all departments', 'draft', 'f0000001-0001-0001-0001-000000000001', 10, 'randomized', 72, '2026-04-01 08:00:00+00', '2026-04-30 17:00:00+00', '2b3b120b-e324-463b-9ae5-70fe5138e368', '{"tls_mode":"auto","post_capture_action":"redirect"}'),
  ('80000001-0001-0001-0001-000000000002', 'Executive Spear Phish', 'Targeted campaign for C-suite executives', 'draft', 'f0000001-0001-0001-0001-000000000003', 2, 'default', 48, '2026-04-15 09:00:00+00', '2026-04-22 17:00:00+00', 'a1111111-1111-1111-1111-111111111111', '{"tls_mode":"auto","post_capture_action":"redirect"}'),
  ('80000001-0001-0001-0001-000000000003', 'IT Password Reset Drill', 'Test IT staff response to password reset phish', 'draft', 'f0000001-0001-0001-0001-000000000002', 20, 'department', 72, '2026-05-01 08:00:00+00', '2026-05-15 17:00:00+00', 'a1111111-1111-1111-1111-111111111111', '{"tls_mode":"auto","post_capture_action":"redirect"}'),
  ('80000001-0001-0001-0001-000000000004', 'Finance Compliance Test', 'Compliance-driven phishing assessment for finance team', 'draft', 'f0000001-0001-0001-0001-000000000001', 5, 'alphabetical', 96, '2026-05-10 08:00:00+00', '2026-05-24 17:00:00+00', '2b3b120b-e324-463b-9ae5-70fe5138e368', '{"tls_mode":"auto","post_capture_action":"display_page"}'),
  ('80000001-0001-0001-0001-000000000005', 'New Hire Security Training', 'Phishing awareness for Q1 new hires', 'draft', 'f0000001-0001-0001-0001-000000000001', 15, 'default', 72, NULL, NULL, 'a2222222-2222-2222-2222-222222222222', '{"tls_mode":"auto"}')
ON CONFLICT DO NOTHING;

-- Campaign template variants
INSERT INTO campaign_template_variants (id, campaign_id, template_id, split_ratio, label) VALUES
  ('90000001-0001-0001-0001-000000000001', '80000001-0001-0001-0001-000000000001', 'e0000001-0001-0001-0001-000000000001', 50, 'Variant A - IT Reset'),
  ('90000001-0001-0001-0001-000000000002', '80000001-0001-0001-0001-000000000001', 'e0000001-0001-0001-0001-000000000005', 50, 'Variant B - M365 Alert'),
  ('90000001-0001-0001-0001-000000000003', '80000001-0001-0001-0001-000000000002', 'e0000001-0001-0001-0001-000000000003', 100, 'DocuSign Spear Phish'),
  ('90000001-0001-0001-0001-000000000004', '80000001-0001-0001-0001-000000000003', 'e0000001-0001-0001-0001-000000000001', 100, 'IT Password Reset'),
  ('90000001-0001-0001-0001-000000000005', '80000001-0001-0001-0001-000000000004', 'e0000001-0001-0001-0001-000000000002', 60, 'Benefits Update'),
  ('90000001-0001-0001-0001-000000000006', '80000001-0001-0001-0001-000000000004', 'e0000001-0001-0001-0001-000000000004', 40, 'Slack Notification'),
  ('90000001-0001-0001-0001-000000000007', '80000001-0001-0001-0001-000000000005', 'e0000001-0001-0001-0001-000000000005', 100, 'M365 Security Alert')
ON CONFLICT DO NOTHING;

-- Campaign send windows (days is JSONB array)
INSERT INTO campaign_send_windows (id, campaign_id, days, start_time, end_time, timezone) VALUES
  (gen_random_uuid(), '80000001-0001-0001-0001-000000000001', '["monday","tuesday","wednesday","thursday","friday"]', '09:00', '17:00', 'America/New_York'),
  (gen_random_uuid(), '80000001-0001-0001-0001-000000000002', '["tuesday","wednesday","thursday"]', '10:00', '15:00', 'America/New_York'),
  (gen_random_uuid(), '80000001-0001-0001-0001-000000000003', '["monday","tuesday","wednesday","thursday","friday"]', '08:00', '18:00', 'America/Chicago')
ON CONFLICT DO NOTHING;

-- ---- Audit Log Entries (source_ip is inet, correlation_id is uuid) ----
INSERT INTO audit_logs (id, category, severity, actor_type, actor_id, actor_label, action, resource_type, resource_id, details, source_ip, correlation_id, created_at) VALUES
  (gen_random_uuid(), 'user_activity', 'info', 'user', '2b3b120b-e324-463b-9ae5-70fe5138e368', 'admin', 'user.login', 'user', '2b3b120b-e324-463b-9ae5-70fe5138e368', '{"method":"local"}', '10.0.1.50', gen_random_uuid(), now() - interval '2 hours'),
  (gen_random_uuid(), 'user_activity', 'info', 'user', 'a1111111-1111-1111-1111-111111111111', 'sarah.chen', 'user.login', 'user', 'a1111111-1111-1111-1111-111111111111', '{"method":"local"}', '10.0.1.51', gen_random_uuid(), now() - interval '1 hour'),
  (gen_random_uuid(), 'user_activity', 'info', 'user', '2b3b120b-e324-463b-9ae5-70fe5138e368', 'admin', 'campaign.created', 'campaign', '80000001-0001-0001-0001-000000000001', '{"name":"Q1 Baseline Assessment"}', '10.0.1.50', gen_random_uuid(), now() - interval '50 minutes'),
  (gen_random_uuid(), 'user_activity', 'info', 'user', 'a1111111-1111-1111-1111-111111111111', 'sarah.chen', 'campaign.created', 'campaign', '80000001-0001-0001-0001-000000000002', '{"name":"Executive Spear Phish"}', '10.0.1.51', gen_random_uuid(), now() - interval '45 minutes'),
  (gen_random_uuid(), 'user_activity', 'info', 'user', '2b3b120b-e324-463b-9ae5-70fe5138e368', 'admin', 'target.bulk_import', 'target', NULL, '{"count":25,"source":"csv"}', '10.0.1.50', gen_random_uuid(), now() - interval '40 minutes'),
  (gen_random_uuid(), 'user_activity', 'warning', 'user', 'a2222222-2222-2222-2222-222222222222', 'mike.ross', 'user.login_failed', 'user', 'a2222222-2222-2222-2222-222222222222', '{"reason":"invalid_password","attempt":2}', '10.0.2.100', gen_random_uuid(), now() - interval '30 minutes'),
  (gen_random_uuid(), 'system', 'info', 'system', NULL, 'system', 'system.startup', NULL, NULL, '{"version":"dev","addr":":8080"}', NULL, gen_random_uuid(), now() - interval '3 hours'),
  (gen_random_uuid(), 'user_activity', 'info', 'user', '2b3b120b-e324-463b-9ae5-70fe5138e368', 'admin', 'settings.updated', 'settings', NULL, '{"fields":["password_min_length","session_timeout"]}', '10.0.1.50', gen_random_uuid(), now() - interval '20 minutes')
ON CONFLICT DO NOTHING;

-- ---- Notifications for admin ----
INSERT INTO notifications (id, user_id, category, severity, title, body, is_read, created_at) VALUES
  (gen_random_uuid(), '2b3b120b-e324-463b-9ae5-70fe5138e368', 'campaign', 'info', 'Campaign Created', 'Campaign "Q1 Baseline Assessment" has been created and is in draft state.', false, now() - interval '50 minutes'),
  (gen_random_uuid(), '2b3b120b-e324-463b-9ae5-70fe5138e368', 'security', 'warning', 'Failed Login Attempt', 'User mike.ross had 2 failed login attempts from 10.0.2.100.', false, now() - interval '30 minutes'),
  (gen_random_uuid(), '2b3b120b-e324-463b-9ae5-70fe5138e368', 'system', 'info', 'System Update', 'All 59 database migrations applied successfully.', true, now() - interval '3 hours'),
  (gen_random_uuid(), '2b3b120b-e324-463b-9ae5-70fe5138e368', 'campaign', 'info', 'New Campaign', 'Sarah Chen created campaign "Executive Spear Phish".', false, now() - interval '45 minutes')
ON CONFLICT DO NOTHING;

-- ---- Summary ----
SELECT 'Seed complete: ' ||
  (SELECT count(*) FROM users) || ' users, ' ||
  (SELECT count(*) FROM targets) || ' targets, ' ||
  (SELECT count(*) FROM target_groups) || ' groups, ' ||
  (SELECT count(*) FROM campaigns) || ' campaigns, ' ||
  (SELECT count(*) FROM email_templates) || ' email templates, ' ||
  (SELECT count(*) FROM landing_page_projects) || ' landing pages, ' ||
  (SELECT count(*) FROM smtp_profiles) || ' SMTP profiles, ' ||
  (SELECT count(*) FROM audit_logs) || ' audit logs, ' ||
  (SELECT count(*) FROM notifications) || ' notifications';
