-- Restore seed landing page f0000001-0001-0001-0001-000000000001 (Corporate Login Portal)
-- This was overwritten by curl testing during debugging on 2026-03-30.
-- Run: psql -U tackle -d tackle -f scripts/restore_seed_lp1.sql

DELETE FROM landing_page_projects WHERE id = 'f0000001-0001-0001-0001-000000000001';

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
  }$LP1$, '2b3b120b-e324-463b-9ae5-70fe5138e368', 'redirect');
