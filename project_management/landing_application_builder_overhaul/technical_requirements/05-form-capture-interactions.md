# 05 — Form Capture & Interaction System

## 5.1 Overview

The form capture system is the primary data collection mechanism of landing applications. When a target submits a form on the landing page, the Go backend captures all form field data, categorizes it, and forwards it upstream to Tackle — all invisible to the target. From the target's perspective, they submitted a form to a normal-looking URL and were redirected normally.

This document covers the capture pipeline from form submission to Tackle ingestion, including field categorization, post-capture actions, and the operator's configuration interface.

## 5.2 Capture Pipeline

```
Target submits form
        │
        ▼
┌─────────────────────────┐
│  Browser sends POST      │  Form action: /signin (operator-defined)
│  to landing app          │  Content-Type: application/x-www-form-urlencoded
│  (via phishing endpoint) │  Body: email=user@corp.com&password=hunter2
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│  Landing App Go Handler  │
│                          │
│  1. Parse form fields    │
│  2. Categorize fields    │  Auto-detect + operator overrides
│  3. Collect metadata     │  IP, User-Agent, headers, timestamp
│  4. Package capture      │
└────────────┬────────────┘
             │
             ├──────────────────────┐
             ▼                      ▼
┌─────────────────────┐  ┌─────────────────────┐
│  POST to Tackle      │  │  POST to Tackle      │
│  /internal/captures  │  │  /internal/metrics   │
│                      │  │                      │
│  {                   │  │  {                   │
│    fields: [...],    │  │    event: "form_sub", │
│    metadata: {...},  │  │    page: "/signin",  │
│    page_route: "..." │  │    timestamp: "..."  │
│  }                   │  │  }                   │
└─────────────────────┘  └─────────────────────┘
             │
             ▼
┌─────────────────────────┐
│  Execute post-capture    │  Navigate to page, redirect,
│  action                  │  show message, replay, etc.
│                          │
│  Returns response to     │
│  browser                 │
└─────────────────────────┘
```

## 5.3 Capture Data Structure

Each captured form submission produces a capture event:

```
CaptureEvent {
    // Identification
    landing_app_id   : string       // Which landing application
    campaign_id      : string       // Which campaign (if production)
    build_id         : string       // Which build
    page_route       : string       // Which page the form was on
    form_action      : string       // The form's action path

    // Captured Fields
    fields           : array        // Array of CaptureField
    
    // Metadata
    source_ip        : string       // Target's IP address
    user_agent       : string       // Target's User-Agent header
    request_headers  : object       // All HTTP headers from the request
    timestamp        : datetime     // When the submission occurred
    tracking_token   : string       // (If present) Per-target tracking identifier
}

CaptureField {
    field_name       : string       // HTML input name attribute
    field_value      : string       // Submitted value
    capture_tag      : string       // Categorization (username, password, email, etc.)
    field_type       : string       // HTML input type (text, password, email, etc.)
}
```

## 5.4 Field Categorization

### Auto-Detection Rules

When the `servergen` pipeline compiles the landing application, each form input component's capture tag is determined by:

**Priority 1 — Operator override**: If the operator explicitly set a capture tag in the builder's Advanced tab, use that.

**Priority 2 — Input type inference**:
| Input Type | Auto-Tag |
|-----------|----------|
| `password` | password |
| `email` | email |
| `hidden` | hidden |
| All others | (fall through to Priority 3) |

**Priority 3 — Name pattern matching**:
| Name Pattern | Auto-Tag |
|-------------|----------|
| user, username, login, userid, account | username |
| pass, password, pwd, passwd | password |
| email, mail, e-mail | email |
| otp, mfa, token, code, verify, 2fa | mfa_token |
| card, cc, credit, cardnum | credit_card |
| (no match) | generic |

Pattern matching is case-insensitive and checks for substring containment.

### Operator Override in Builder

In the builder's Advanced tab, each input component within a form displays:

```
Capture Tag: [auto-detected value ▼]
             ├── username
             ├── password
             ├── email
             ├── mfa_token
             ├── credit_card
             ├── custom
             └── generic
```

The auto-detected value is shown as the default. Operator can select a different tag.

## 5.5 Post-Capture Actions

After the Go backend captures the form data and forwards it to Tackle, it executes the operator-configured post-capture action. These are configured per-form in the builder.

### Navigate to Page

Route the target to another page within the landing application.

```
Configuration:
  action: "navigate_to_page"
  target_page: "/mfa"          // Internal page route
```

**Implementation**: The Go handler returns a JSON response `{ "redirect": "/mfa" }` and the React frontend navigates client-side. Alternatively for non-JS scenarios, the Go handler returns an HTTP 302 redirect.

### External Redirect

Redirect the target to an external URL.

```
Configuration:
  action: "external_redirect"
  target_url: "https://login.microsoft.com"
```

**Implementation**: The Go handler returns an HTTP 302 redirect to the external URL.

### Display Message

Show a message on the current page without navigating away.

```
Configuration:
  action: "display_message"
  message: "Your account has been verified."
  message_style: "success"     // success, info, warning, error
```

**Implementation**: The Go handler returns a JSON response with the message. The React frontend displays it as an alert/toast component.

### Delayed Redirect

Show a loading state (spinner, progress bar, or message) for a configured duration, then redirect.

```
Configuration:
  action: "delayed_redirect"
  delay_ms: 3000
  loading_message: "Verifying your credentials..."
  target: "/dashboard"         // Internal page or external URL
```

**Implementation**: The Go handler returns a response instructing the React frontend to show the loading state and redirect after the delay.

### Replay to Real Service

Forward the submitted form data to the real service being spoofed, so the target's login actually succeeds on the real site.

```
Configuration:
  action: "replay_submission"
  replay_url: "https://login.microsoftonline.com/common/oauth2/token"
  replay_method: "POST"
```

**Implementation**: The Go handler first captures the data for Tackle, then makes an HTTP request to the replay URL with the same form data. The response from the real service is forwarded back to the target's browser.

### No Action

The form submits silently. The page does not navigate or change. Useful when the form's submit handler is managed by a separate workflow flow.

```
Configuration:
  action: "no_action"
```

## 5.6 Multiple Forms Per Page

A single page can contain multiple forms, each with independent capture configuration:

- Each form has its own action path (e.g., one form POSTs to `/signin`, another to `/register`)
- Each form has its own post-capture action
- Each form's inputs have independent capture tags
- The Go backend generates a separate handler for each form action path

## 5.7 Capture Without Forms

Not all data capture requires a visible form submission. The following mechanisms capture data without the target explicitly submitting a form:

| Mechanism | Trigger | Data Captured | Configuration |
|-----------|---------|---------------|---------------|
| **Keylogging** | Keystroke on targeted input | Character-by-character input | Page-level or component-level behavior (see doc 06) |
| **Clipboard** | Paste event on targeted input | Clipboard contents | Component-level behavior |
| **Session tokens** | Page load | localStorage, sessionStorage, cookies | Page-level behavior |
| **Browser fingerprint** | Page load | Hardware, browser, screen info | Page-level behavior |

These are covered in detail in [06 - Behavioral Capabilities](06-behavioral-capabilities.md).

## 5.8 Request Metadata Collection

Every form submission captures metadata about the HTTP request, independent of form fields:

| Metadata | Source | Purpose |
|----------|--------|---------|
| Source IP | Request remote address | Geographic location, VPN detection |
| User-Agent | `User-Agent` header | Browser and OS identification |
| Referer | `Referer` header | How the target arrived |
| Accept-Language | `Accept-Language` header | Target's language preferences |
| All headers | Full header map | Complete request fingerprint |
| Timestamp | Server clock | When the submission occurred |
| Page route | Handler context | Which page the form was on |
| Tracking token | Cookie or URL parameter | Per-target campaign tracking |

## 5.9 Operator Configuration Summary

From the builder UI, the operator configures capture behavior at three levels:

### Form Level (Form component Advanced tab)

- **Action path**: What URL the form POSTs to (visible to target)
- **Post-capture action**: What happens after submission (navigate, redirect, message, etc.)
- **Replay URL**: (If replay action) Where to forward the form data

### Field Level (Input component Advanced tab)

- **Capture tag**: How the field is categorized (auto-detected, operator can override)
- **Field name**: The HTML name attribute (used in form submission and capture records)

### Page Level (Page behaviors)

- **Behavioral capabilities**: Keylogging, session capture, fingerprinting (see doc 06)

## 5.10 Capture Endpoint Generation

The `servergen` pipeline generates Go route handlers for each unique form action path defined in the project:

1. Scans all pages for Form components
2. Collects all unique action paths
3. For each action path, generates a Go handler that:
   - Parses the form body
   - Maps field names to capture tags (based on the builder configuration)
   - Constructs the CaptureEvent payload
   - POSTs the payload to Tackle's `/api/v1/internal/captures` endpoint
   - Fires a `form_submission` metric event to Tackle's `/api/v1/internal/metrics` endpoint
   - Executes the post-capture action (redirect, navigate, display, etc.)

### Handler Example (conceptual)

For a form with action `/signin` and post-capture action "navigate to /mfa":

```
POST /signin
  → Parse form body: { email: "...", password: "..." }
  → Build capture payload with field tags
  → POST capture to Tackle internal API
  → POST form_submission metric to Tackle internal API
  → Return JSON: { "navigate": "/mfa" }
```

## 5.11 Security Considerations

### No Client-Side Exfiltration

Form data is captured entirely on the Go backend. There is no JavaScript-based interception of form submissions for the purpose of capture. The form POSTs to a Go handler; the Go handler captures the data and communicates with Tackle. This means:

- No suspicious JavaScript in the page source
- No XHR/fetch calls to unexpected endpoints visible in browser dev tools
- The form submission looks completely normal to the target and to defensive tooling inspecting the browser

### Data in Transit

- Target → Landing App: Encrypted via TLS (the phishing endpoint terminates TLS)
- Landing App → Tackle: Local network communication (same host), no encryption needed
- Tackle encrypts captured credentials at rest using AES-256-GCM
