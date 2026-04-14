# 10 — Telemetry & Upstream Communication

## 10.1 Overview

Every running landing application communicates upstream to the Tackle framework to report two categories of data:

1. **Captures**: Form field data, keystroke data, clipboard data, session tokens — the "loot" from target interactions
2. **Metrics**: Behavioral events, page views, downloads, timing data — interaction analytics

This communication is a one-way data flow: the landing application POSTs data to Tackle's internal API. Tackle never pushes data to a production landing application (dev server HMR updates are the exception, covered in doc 09).

All upstream communication occurs over the local network on the Tackle server. It is never exposed to the internet, never visible to targets, and never visible to defensive tooling.

## 10.2 Internal API Endpoints

Tackle exposes two internal API endpoints for landing application communication:

### Captures Endpoint

```
POST /api/v1/internal/captures
```

Receives captured form data, keystroke data, clipboard data, and session tokens.

### Metrics Endpoint

```
POST /api/v1/internal/metrics
```

Receives behavioral events, page views, downloads, timing data, and interaction analytics.

### Internal Routing

These endpoints are registered on Tackle's HTTP server but are:
- **Not** exposed through any public-facing interface
- **Not** protected by authentication (local network trust)
- **Not** subject to the standard API rate limiting (captures must not be dropped)
- Bound to localhost or the internal network interface only

## 10.3 Capture Payload Format

### Form Submission Capture

```json
{
    "type": "form_submission",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "form_action": "/signin",
    "timestamp": "2026-04-14T15:30:00Z",
    "fields": [
        {
            "name": "email",
            "value": "user@corp.com",
            "capture_tag": "email",
            "input_type": "email"
        },
        {
            "name": "password",
            "value": "hunter2",
            "capture_tag": "password",
            "input_type": "password"
        }
    ],
    "metadata": {
        "source_ip": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "referer": "https://phishing-endpoint.com/signin",
        "accept_language": "en-US,en;q=0.9",
        "headers": {
            "Accept": "text/html...",
            "Connection": "keep-alive"
        },
        "tracking_token": "abc123"
    }
}
```

### Keystroke Capture

```json
{
    "type": "keystroke_capture",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:30:01Z",
    "field_name": "password",
    "keystrokes": [
        { "key": "h", "timestamp": "2026-04-14T15:30:00.100Z" },
        { "key": "u", "timestamp": "2026-04-14T15:30:00.250Z" },
        { "key": "n", "timestamp": "2026-04-14T15:30:00.400Z" }
    ],
    "cumulative_value": "hun",
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Clipboard Capture

```json
{
    "type": "clipboard_capture",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:30:02Z",
    "field_name": "password",
    "pasted_value": "MyP@ssw0rd!",
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Session Capture

```json
{
    "type": "session_capture",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:30:03Z",
    "session_data": {
        "cookies": {
            "session_id": "abc123def456",
            "auth_token": "eyJ..."
        },
        "local_storage": {
            "user_prefs": "{\"theme\":\"dark\"}"
        },
        "session_storage": {
            "temp_data": "value"
        }
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

## 10.4 Metrics Payload Format

### Page View Event

```json
{
    "event_type": "page_view",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:29:55Z",
    "data": {
        "referrer": "https://email-link.com/track",
        "load_time_ms": 450
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "tracking_token": "abc123"
    }
}
```

### Form Submission Event

```json
{
    "event_type": "form_submission",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "form_action": "/signin",
    "timestamp": "2026-04-14T15:30:00Z",
    "data": {
        "field_count": 2,
        "capture_tags": ["email", "password"]
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### File Download Event

```json
{
    "event_type": "file_download",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/download",
    "timestamp": "2026-04-14T15:31:00Z",
    "data": {
        "file_id": "uuid",
        "filename": "Q4-Report.pdf",
        "content_type": "application/pdf",
        "size_bytes": 1048576,
        "is_payload": true
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Browser Fingerprint Event

```json
{
    "event_type": "browser_fingerprint",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:29:56Z",
    "data": {
        "screen_width": 1920,
        "screen_height": 1080,
        "color_depth": 24,
        "pixel_ratio": 1,
        "platform": "Win32",
        "language": "en-US",
        "languages": ["en-US", "en"],
        "hardware_concurrency": 8,
        "device_memory": 16,
        "timezone": "America/New_York",
        "utc_offset": -240,
        "webgl_renderer": "ANGLE (NVIDIA GeForce GTX 1080)",
        "webgl_vendor": "Google Inc. (NVIDIA)",
        "plugins_count": 3,
        "cookie_enabled": true,
        "do_not_track": false,
        "max_touch_points": 0
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Element Click Event

```json
{
    "event_type": "element_click",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/consent",
    "timestamp": "2026-04-14T15:30:05Z",
    "data": {
        "element_id": "allow-btn",
        "element_type": "button",
        "click_x": 150,
        "click_y": 300
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Time on Page Event

```json
{
    "event_type": "time_on_page",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:30:30Z",
    "data": {
        "total_time_ms": 35000,
        "active_time_ms": 30000,
        "idle_periods": 1
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Scroll Depth Event

```json
{
    "event_type": "scroll_depth",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/article",
    "timestamp": "2026-04-14T15:31:30Z",
    "data": {
        "max_depth_percent": 75,
        "checkpoints_reached": ["25", "50", "75"]
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

### Field Interaction Event

```json
{
    "event_type": "field_interaction",
    "project_id": "uuid",
    "campaign_id": "uuid",
    "build_id": "uuid",
    "mode": "production",
    "page_route": "/signin",
    "timestamp": "2026-04-14T15:29:58Z",
    "data": {
        "event": "blur",
        "field_name": "email",
        "field_value": "user@corp.com",
        "focus_duration_ms": 4500
    },
    "metadata": {
        "source_ip": "192.168.1.100",
        "tracking_token": "abc123"
    }
}
```

## 10.5 Tackle-Side Processing

### Capture Ingestion

When Tackle receives a capture POST:

1. Validate the payload structure
2. Look up the project and campaign (if production mode)
3. Encrypt sensitive field values (passwords, tokens) using AES-256-GCM
4. Store the capture event in the `capture_events` table
5. Store individual fields in the `capture_fields` table
6. Update real-time campaign dashboard via WebSocket (if production)
7. Log the capture event in the audit log

### Metrics Ingestion

When Tackle receives a metrics POST:

1. Validate the payload structure
2. Look up the project and campaign
3. Store the metric event
4. Update real-time campaign dashboard via WebSocket (if production)
5. Aggregate into campaign-level statistics

### Development Mode Handling

Captures and metrics with `"mode": "development"` are:
- Stored separately from production data
- Visible in a development capture log (accessible from the builder)
- Excluded from campaign reports and dashboards
- Subject to shorter retention (auto-purged after 7 days)

## 10.6 Error Handling and Resilience

### Landing App Side

If the landing application cannot reach Tackle's internal API:

1. **Buffer in memory**: Capture and metric payloads are queued in a bounded in-memory buffer
2. **Retry with backoff**: Retries every 2 seconds, then 4, 8, 16, up to 60 seconds
3. **Priority**: Capture payloads (form data, keystrokes, sessions) are prioritized over metric payloads
4. **Buffer limits**: Maximum 1000 queued payloads. If the buffer fills, oldest metric payloads are dropped first. Capture payloads are only dropped as a last resort.
5. **No target impact**: The landing application continues serving pages and handling form submissions regardless of upstream connectivity. The target experience is never affected.

### Tackle Side

If Tackle receives malformed payloads:
- Returns HTTP 400 with error details
- Logs the malformed payload for debugging
- Does not crash or reject subsequent valid payloads

If Tackle's database is unavailable:
- Capture payloads are buffered in Tackle's own in-memory queue
- Retried when database connectivity is restored
- Maximum buffer: 10,000 payloads before dropping

## 10.7 Tracking Tokens

Tracking tokens allow Tackle to correlate captures and metrics with specific targets across their entire interaction session.

### Token Assignment

When a target first accesses the landing application (arriving from a phishing email link), the URL may contain a tracking token parameter (e.g., `?t=abc123`). The landing application:

1. Extracts the tracking token from the URL
2. Stores it in a cookie (for persistence across page navigations)
3. Includes the token in all capture and metric payloads

### Token Propagation

The tracking token is included in:
- All form submission captures
- All behavioral event metrics
- All file download events
- All session captures

This allows Tackle to build a complete interaction timeline for each target.

### Token Absence

If no tracking token is present (e.g., the target navigated directly to the landing page without a tracked email link):
- Captures and metrics are still recorded
- The `tracking_token` field is null
- Tackle can still correlate by source IP and user agent (best-effort)

## 10.8 Event Flow Summary

```
Target Action                  Landing App Backend       Tackle Internal API
─────────────                  ───────────────────       ───────────────────
Loads page              →      page_view metric    →     /internal/metrics
                               fingerprint event   →     /internal/metrics
                               session extraction  →     /internal/captures

Types in field          →      keystroke capture   →     /internal/captures
                               field_interaction   →     /internal/metrics

Pastes text             →      clipboard capture   →     /internal/captures

Submits form            →      form capture        →     /internal/captures
                               form_submission     →     /internal/metrics

Clicks button           →      element_click       →     /internal/metrics

Downloads file          →      file_download       →     /internal/metrics

Scrolls page            →      scroll_depth        →     /internal/metrics

Spends time             →      time_on_page        →     /internal/metrics
                               (periodic)
```
