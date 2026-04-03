# 18 — Notification System

## 1. Purpose

The Notification System provides a unified mechanism for delivering alerts, status updates, and event notifications to Tackle users across three channels: in-app notifications, email notifications, and webhook integrations. It is a cross-cutting concern consumed by every feature module — campaign lifecycle events, infrastructure alerts, audit log alerts, block list events, and system health updates all route through this system.

## 2. Notification Channels

### 2.1 In-App Notifications

| Feature | Behavior |
|---------|----------|
| **Notification inbox** | Each user has a persistent notification inbox accessible from the top navigation bar. |
| **Badge count** | An unread notification count badge is displayed on the notification icon. |
| **Real-time delivery** | Notifications are pushed to connected sessions via WebSocket within 3 seconds of the triggering event. |
| **Persistence** | Notifications are stored in the database and persist across sessions until dismissed or expired. |
| **Actions** | Each notification can optionally include a primary action link (e.g., "View Campaign", "Review Approval"). |
| **Read/unread state** | Notifications track read/unread state per user. Bulk "mark all as read" is supported. |
| **Retention** | Notifications are retained for 90 days, then automatically purged. Retention period is configurable. |

### 2.2 Email Notifications

| Feature | Behavior |
|---------|----------|
| **Opt-in delivery** | Email notifications are opt-in per user. Users configure which notification categories they want to receive via email in their user preferences. |
| **SMTP configuration** | Email notifications use a system-level SMTP configuration (separate from campaign SMTP profiles) configured by an Administrator. |
| **Digest mode** | Users can choose between immediate delivery (one email per notification) or digest mode (batched notifications sent at configurable intervals: hourly, daily, or weekly). |
| **Templates** | Notification emails use a standard Tackle-branded template with the notification title, body, action link, and timestamp. |
| **Unsubscribe** | Each notification email includes an unsubscribe link that disables email notifications for that category. |

### 2.3 Webhook Notifications

| Feature | Behavior |
|---------|----------|
| **Webhook endpoints** | Administrators can configure one or more webhook endpoints (URLs) that receive notification payloads via HTTP POST. |
| **Category filtering** | Each webhook endpoint can be configured to receive notifications for specific categories only. |
| **Payload format** | Webhook payloads are JSON objects containing: notification ID, category, severity, title, body, resource type, resource ID, action URL, and timestamp. |
| **Authentication** | Webhook requests include a configurable authentication mechanism: HMAC signature header, bearer token header, or basic auth. |
| **Retry logic** | Failed webhook deliveries are retried up to 3 times with exponential backoff (5s, 30s, 5m). |
| **Delivery log** | All webhook delivery attempts (success and failure) are logged with response status, response body (truncated), and timing. |

## 3. Notification Categories

| Category | Example Events | Default Recipients | Default Channels |
|----------|---------------|-------------------|-----------------|
| **Campaign lifecycle** | Campaign submitted, approved, rejected, launched, completed, archived | Campaign Operator, approvers | In-app |
| **Campaign alerts** | Build failure, endpoint error, SMTP failure, all emails sent | Campaign Operator, Engineers | In-app, email |
| **Approval requests** | Campaign pending approval, block list override request | Engineers, Administrators | In-app, email |
| **Infrastructure** | Endpoint provisioned, endpoint error, endpoint terminated, health check failure | Engineers, Administrators | In-app |
| **Block list** | Target matches block list, override approved/rejected, block list entry added | Campaign Operator, Administrators | In-app, email |
| **Credential capture** | New credential captured (count, not values), campaign capture milestone reached | Campaign Operator | In-app |
| **Audit alerts** | Rule-based audit alerts triggered (see [11-audit-logging.md](11-audit-logging.md)) | Configured per rule | In-app, email, webhook |
| **System** | System startup/shutdown, database migration, configuration change, scheduled maintenance | Administrators | In-app |
| **Report** | Scheduled report generated, report generation failed | Report creator | In-app, email |

## 4. Requirements

### REQ-NOTIF-001: Notification Creation API

The system SHALL provide an internal notification creation API that all feature modules use to emit notifications.

**Acceptance Criteria:**
- [ ] A single internal Go function/service creates notifications with: category, severity, title, body, resource type/ID, action URL, and recipient specification (by user ID, role, or campaign association)
- [ ] The notification service resolves recipients based on the specification and creates per-user notification records
- [ ] Channel delivery (in-app, email, webhook) is determined by user preferences and webhook configurations
- [ ] Notification creation is asynchronous and does not block the calling operation

---

### REQ-NOTIF-002: In-App Notification UI

The system SHALL provide an in-app notification interface in the admin UI.

**Acceptance Criteria:**
- [ ] A notification bell icon in the top navigation bar displays the unread notification count
- [ ] Clicking the bell opens a dropdown panel showing the most recent 20 notifications
- [ ] Each notification shows: category icon, title, body preview (truncated), timestamp (relative, e.g., "5 minutes ago"), and read/unread indicator
- [ ] Clicking a notification navigates to the relevant resource (campaign, endpoint, report, etc.)
- [ ] A "View All" link opens a full notification inbox page with filtering and search
- [ ] The full inbox supports filtering by category, read/unread state, and date range
- [ ] Bulk operations: "Mark all as read", "Delete selected", "Delete all read"

---

### REQ-NOTIF-003: Email Notification Delivery

The system SHALL deliver email notifications to users who have opted in for email delivery.

**Acceptance Criteria:**
- [ ] Email notifications are sent using a system-level SMTP configuration (not campaign SMTP profiles)
- [ ] Users configure email notification preferences per category in their user profile settings
- [ ] Immediate mode: email sent within 60 seconds of notification creation
- [ ] Digest mode: notifications batched and sent at the configured interval (hourly, daily, weekly)
- [ ] Email template includes: notification title, body, action link (button), timestamp, and unsubscribe link
- [ ] Unsubscribe link disables email notifications for that specific category without affecting other categories
- [ ] Email delivery failures are logged but do not prevent in-app notification delivery

---

### REQ-NOTIF-004: Webhook Delivery

The system SHALL deliver notifications to configured webhook endpoints.

**Acceptance Criteria:**
- [ ] Administrators can configure webhook endpoints via the settings UI
- [ ] Each webhook endpoint has: name, URL, authentication method, category filter, enabled/disabled toggle
- [ ] Webhook payloads are JSON with a consistent schema documented in the API spec
- [ ] HMAC signature authentication uses SHA-256 with a configurable secret key
- [ ] Failed deliveries are retried up to 3 times with exponential backoff
- [ ] A webhook delivery log is viewable in the admin UI showing recent delivery attempts and their outcomes
- [ ] Webhook endpoints can be tested with a "Send Test" button that delivers a sample notification

---

### REQ-NOTIF-005: User Notification Preferences

The system SHALL allow users to configure their notification preferences.

**Acceptance Criteria:**
- [ ] User preferences are accessible from the user profile / settings page
- [ ] Users can enable/disable email notifications per category
- [ ] Users can select email delivery mode: immediate or digest (with interval selection)
- [ ] Users cannot disable in-app notifications (always delivered)
- [ ] Administrators can set organization-wide default preferences for new users
- [ ] Preference changes take effect immediately

---

### REQ-NOTIF-006: Notification Severity

Each notification SHALL carry a severity level that affects display and delivery behavior.

| Severity | Display | Email Behavior |
|----------|---------|----------------|
| **Info** | Standard notification | Included in digests only (unless user preference overrides) |
| **Warning** | Yellow/amber indicator | Sent immediately even in digest mode |
| **Critical** | Red indicator, persistent until acknowledged | Sent immediately, bypasses all preferences (always delivered to in-app + email) |

**Acceptance Criteria:**
- [ ] Info notifications are styled as standard messages
- [ ] Warning notifications have a visual amber/yellow indicator
- [ ] Critical notifications have a visual red indicator and remain visible until explicitly acknowledged
- [ ] Critical notifications bypass digest mode and user opt-out (except unsubscribed webhook endpoints)

---

## 5. API Endpoints

| Method | Endpoint | Description | Minimum Role |
|--------|----------|-------------|-------------|
| GET | `/api/v1/notifications` | List current user's notifications (paginated, filterable) | Any authenticated |
| PUT | `/api/v1/notifications/:id/read` | Mark a notification as read | Any authenticated |
| POST | `/api/v1/notifications/read-all` | Mark all notifications as read | Any authenticated |
| DELETE | `/api/v1/notifications/:id` | Delete a notification | Any authenticated |
| GET | `/api/v1/notifications/preferences` | Get current user's notification preferences | Any authenticated |
| PUT | `/api/v1/notifications/preferences` | Update notification preferences | Any authenticated |
| GET | `/api/v1/webhooks` | List webhook configurations | Administrator |
| POST | `/api/v1/webhooks` | Create a webhook endpoint | Administrator |
| PUT | `/api/v1/webhooks/:id` | Update a webhook endpoint | Administrator |
| DELETE | `/api/v1/webhooks/:id` | Delete a webhook endpoint | Administrator |
| POST | `/api/v1/webhooks/:id/test` | Send a test notification to webhook | Administrator |
| GET | `/api/v1/webhooks/:id/deliveries` | View webhook delivery log | Administrator |
| GET | `/api/v1/admin/notifications/smtp` | Get system notification SMTP config | Administrator |
| PUT | `/api/v1/admin/notifications/smtp` | Update system notification SMTP config | Administrator |

## 6. Database Entities

| Entity | Key Fields | Purpose |
|--------|------------|---------|
| `notifications` | `id`, `user_id`, `category`, `severity`, `title`, `body`, `resource_type`, `resource_id`, `action_url`, `is_read`, `created_at`, `expires_at` | Per-user notification records |
| `notification_preferences` | `id`, `user_id`, `category`, `email_enabled`, `email_mode` (immediate/digest), `digest_interval` | Per-user per-category delivery preferences |
| `webhook_endpoints` | `id`, `name`, `url`, `auth_type`, `auth_config` (encrypted), `categories` (array), `is_enabled`, `created_by`, `created_at` | Webhook endpoint configurations |
| `webhook_deliveries` | `id`, `webhook_id`, `notification_id`, `status` (success/failed), `response_code`, `response_body` (truncated), `attempted_at`, `retry_count` | Webhook delivery attempt log |
| `notification_smtp_config` | `id`, `host`, `port`, `auth_type`, `username`, `password` (encrypted), `tls_mode`, `from_address`, `from_name` | System-level SMTP for notification emails |

## 7. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Notification content exposure | Notifications never contain credential values, passwords, or tokens. Sensitive data is referenced by ID only. |
| Webhook secret exposure | Webhook auth secrets are encrypted at rest and never included in API GET responses. |
| Email notification spoofing | System notification emails use the configured system SMTP with proper authentication. |
| Notification spam | Rate limiting on notification creation prevents flooding (configurable, default: 100 notifications per user per hour). |
| Webhook SSRF | Webhook URLs are validated to prevent Server-Side Request Forgery (no loopback addresses, no private networks unless explicitly allowed by an Administrator). |

## 8. Acceptance Criteria

- [ ] In-app notifications appear within 3 seconds of the triggering event via WebSocket
- [ ] The notification bell displays an accurate unread count
- [ ] Email notifications are delivered within 60 seconds (immediate mode) or at the next scheduled interval (digest mode)
- [ ] Webhook deliveries include proper authentication headers and retry on failure
- [ ] Users can configure per-category notification preferences
- [ ] Critical notifications bypass user opt-out and digest mode
- [ ] Webhook delivery logs are viewable by Administrators
- [ ] Notification retention (auto-purge after 90 days) works correctly
- [ ] All notification delivery operations are logged in the audit trail

## 9. Dependencies

| Dependency | Document | Nature |
|------------|----------|--------|
| Authentication & RBAC | [02-authentication-authorization.md](02-authentication-authorization.md) | User identity for notification targeting, role-based recipient resolution |
| Campaign Management | [06-campaign-management.md](06-campaign-management.md) | Campaign lifecycle events trigger notifications |
| Phishing Endpoints | [07-phishing-endpoints.md](07-phishing-endpoints.md) | Infrastructure events trigger notifications |
| Target Management | [09-target-management.md](09-target-management.md) | Block list events trigger notifications |
| Audit Logging | [11-audit-logging.md](11-audit-logging.md) | Rule-based alerts use the notification system for delivery |
| Database Schema | [14-database-schema.md](14-database-schema.md) | Notification entity definitions |
| Frontend Architecture | [16-frontend-architecture.md](16-frontend-architecture.md) | Notification UI components |
