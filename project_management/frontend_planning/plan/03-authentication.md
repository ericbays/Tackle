# 03 — Authentication & Authorization UI

This document specifies the authentication flow, setup wizard, token management, and permission-based rendering patterns for the Tackle admin UI.

---

## 1. Application Boot Sequence

When the application loads, the following sequence determines what the user sees:

1. **Check setup status**: `GET /api/v1/setup/status` — if setup is not complete, render the Setup Wizard (section 3).
2. **Check for existing session**: If an access token exists in memory, validate it by calling `GET /api/v1/auth/me`.
3. **Token valid**: Redirect to the dashboard (or the originally requested URL if the user was redirected from a protected route).
4. **Token invalid/missing**: Attempt a silent refresh via `POST /api/v1/auth/refresh` (the refresh token is in an HTTP-only cookie).
5. **Refresh succeeds**: Store new access token in memory, proceed to step 3.
6. **Refresh fails**: Render the Login Page (section 2).

During steps 1–5, a full-screen loading state is shown: the Tackle logo centered on a `--bg-primary` background with a subtle pulse animation.

---

## 2. Login Page

### 2.1 Layout

The login page is a standalone layout — no sidebar, no top bar. It uses `--bg-primary` as the full-page background.

```
┌──────────────────────────────────────────────────────────┐
│                                                          │
│                                                          │
│              ┌──────────────────────────┐                │
│              │                          │                │
│              │   [Tackle Logo, 64px]    │                │
│              │       TACKLE             │                │
│              │                          │                │
│              │   Username or Email      │                │
│              │   [____________________] │                │
│              │                          │                │
│              │   Password               │                │
│              │   [____________________] │                │
│              │                          │                │
│              │   [      Log In        ] │                │
│              │                          │                │
│              │   ── or continue with ── │                │
│              │                          │                │
│              │   [ OIDC Provider Name ] │                │
│              │   [ LDAP Provider Name ] │                │
│              │                          │                │
│              └──────────────────────────┘                │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### 2.2 Login Form Behavior

- **Fields**: Username/email (text input) and password (password input with show/hide toggle).
- **Client-side validation**: Both fields must be non-empty. Validation fires on submit, not on blur.
- **Submit**: `POST /api/v1/auth/login` with `{ username, password }`.
- **Loading state**: The "Log In" button shows a spinner and is disabled during the API call.
- **Success**: Store the access token in memory (JavaScript variable, not localStorage/sessionStorage). The refresh token is set as an HTTP-only cookie by the backend. Redirect to the dashboard or the originally requested URL.
- **Failure**: Display a generic error message above the form: "Invalid credentials. Please try again." — never reveal whether the username or password was incorrect.
- **Rate limit exceeded**: Display: "Too many login attempts. Please try again in X minutes." The rate limit headers from the API response (`X-RateLimit-Reset`) determine the countdown.
- **Account locked**: Display: "Account locked. Contact your administrator."

### 2.3 External Auth Providers

- External provider buttons are fetched dynamically from `GET /api/v1/auth/providers`.
- Only enabled providers are rendered.
- Each provider button displays the provider's configured `name` field.
- The divider "or continue with" only appears if at least one external provider is enabled.
- Clicking an OIDC/FusionAuth provider button calls `GET /api/v1/auth/oidc/{providerID}/login`, which redirects the browser to the external provider's login page.
- After external authentication, the callback redirects back to the application with tokens.
- Clicking an LDAP provider button does not redirect — instead it changes the login form's submit handler to authenticate via LDAP through the backend.

### 2.4 Force Password Change

If the API response includes `force_password_change: true`, the login page transitions to a "Change Password" form instead of redirecting to the dashboard:

```
┌──────────────────────────┐
│  Password Change Required│
│                          │
│  You must set a new      │
│  password before         │
│  continuing.             │
│                          │
│  New Password            │
│  [____________________]  │
│                          │
│  Confirm Password        │
│  [____________________]  │
│                          │
│  (password requirements  │
│   listed here)           │
│                          │
│  [  Change Password    ] │
└──────────────────────────┘
```

- Password requirements are fetched from system settings and displayed as a checklist.
- Each requirement shows a green checkmark when satisfied, gray when not.
- The submit button is disabled until all requirements are met and passwords match.

---

## 3. Setup Wizard

### 3.1 When It Appears

The setup wizard appears when `GET /api/v1/setup/status` returns `{ "setup_complete": false }`. This happens exactly once — on a fresh installation before any users exist.

### 3.2 Layout

Same standalone layout as the login page (no sidebar, no top bar). A single centered card.

```
┌──────────────────────────┐
│                          │
│   [Tackle Logo, 64px]   │
│   Welcome to TACKLE      │
│                          │
│   Create your admin      │
│   account to get started │
│                          │
│   Username               │
│   [____________________] │
│                          │
│   Email                  │
│   [____________________] │
│                          │
│   Display Name           │
│   [____________________] │
│                          │
│   Password               │
│   [____________________] │
│   (strength meter bar)   │
│   ☐ 12+ characters       │
│   ☐ Uppercase letter      │
│   ☐ Digit                 │
│   ☐ Special character     │
│                          │
│   Confirm Password       │
│   [____________________] │
│                          │
│   [ Create Account     ] │
│                          │
└──────────────────────────┘
```

### 3.3 Behavior

- **Password strength meter**: A colored bar that fills as the password meets requirements. Colors: red (weak), yellow (moderate), green (strong). Updates in real time as the user types.
- **Requirement checklist**: Each requirement toggles from gray unchecked to green checked as the password satisfies it.
- **Confirm password**: Validated on blur. Shows inline error "Passwords do not match" if different.
- **Submit**: `POST /api/v1/setup` with `{ username, email, display_name, password }`.
- **Success**: The user is automatically logged in (tokens returned in the setup response) and redirected to the dashboard.
- **Error**: Display the error message from the API inline above the form.
- **After completion**: The setup wizard never appears again. If someone navigates to the setup URL after setup is complete, they are redirected to the login page.

---

## 4. Token Management

### 4.1 Access Token Storage

- The JWT access token is stored **in memory only** (a JavaScript variable / Zustand store). It is never written to `localStorage`, `sessionStorage`, or cookies.
- This limits XSS exposure — if the tab is closed, the token is lost and a refresh is required on next visit.

### 4.2 Refresh Token

- The refresh token is stored in an HTTP-only, Secure, SameSite=Strict cookie set by the backend.
- The frontend never directly reads or manipulates the refresh token value.
- Token refresh is initiated by calling `POST /api/v1/auth/refresh` — the browser automatically sends the cookie.

### 4.3 Automatic Token Refresh

- When any API request receives a `401 Unauthorized` response, the API client interceptor:
  1. Pauses the failed request.
  2. Calls `POST /api/v1/auth/refresh`.
  3. If refresh succeeds: stores the new access token, retries the original request with the new token.
  4. If refresh fails: clears the access token, redirects to the login page with a "Session expired. Please log in again." message.

- **Request queuing**: If multiple API requests fail with 401 simultaneously, only one refresh call is made. All pending requests wait for the refresh result and then retry with the new token.

### 4.4 Logout

- `POST /api/v1/auth/logout` revokes the current tokens.
- The frontend clears the in-memory access token, all TanStack Query caches, and all Zustand stores.
- The user is redirected to the login page.
- The WebSocket connection is closed.

---

## 5. Permission-Based UI Rendering

### 5.1 Permission Source

Permissions are extracted from the JWT claims (`permissions` array) or fetched from `GET /api/v1/auth/me`. The permission set is stored in a Zustand auth store and made available to all components.

### 5.2 usePermissions Hook

A `usePermissions()` hook provides:

```typescript
const { hasPermission, hasAnyPermission, hasAllPermissions, role, isAdmin } = usePermissions();

// Usage
hasPermission('campaigns:create')       // boolean
hasAnyPermission(['campaigns:read', 'campaigns:create'])  // boolean
hasAllPermissions(['users:read', 'users:create'])         // boolean
isAdmin                                  // boolean (admin role bypasses all checks)
```

### 5.3 PermissionGate Component

A wrapper component that conditionally renders children based on permissions:

```tsx
<PermissionGate permission="campaigns:create">
  <Button>New Campaign</Button>
</PermissionGate>

<PermissionGate permission="campaigns:delete" fallback={null}>
  <Button variant="danger">Delete</Button>
</PermissionGate>
```

- When the user lacks the required permission, the children are **not rendered** (removed from the DOM entirely).
- The optional `fallback` prop renders alternative content (default: renders nothing).

### 5.4 Rendering Rules

| Scenario | Behavior |
|----------|----------|
| User lacks read permission for an entity | The entire navigation item and page are not rendered. Navigating to the URL directly shows a "403 — Access Denied" page. |
| User has read but not write permission | The page renders in read-only mode. Edit buttons, form inputs, and action buttons are not rendered. |
| Destructive actions (delete, terminate) | Hidden entirely for users without the required permission. Never shown as disabled. |
| Admin role | Bypasses all frontend permission checks. All UI elements are visible. |

### 5.5 Permission Refresh

- When the access token is refreshed, the new permission set from the JWT replaces the previous one.
- Any permission-gated components re-evaluate immediately.
- If permissions were reduced (e.g., role changed by an admin), UI elements disappear on the next token refresh without requiring a page reload.

---

## 6. Session Expiration

### 6.1 Idle Timeout Handling

- The frontend does not implement its own idle timer. Session expiration is enforced by the backend via token lifetimes.
- When a token refresh fails (refresh token expired), the user sees:

```
┌──────────────────────────────────────┐
│                                      │
│   (Clock icon)                       │
│                                      │
│   Session Expired                    │
│                                      │
│   Your session has expired.          │
│   Please log in again.              │
│                                      │
│   [Log In]                          │
│                                      │
└──────────────────────────────────────┘
```

- This message appears as a full-page overlay (not replacing the current page content), so the user can see where they were before the session expired.
- Clicking "Log In" redirects to the login page with the current URL stored as a redirect parameter.

### 6.2 Concurrent Session Limit

- If the backend enforces a max concurrent sessions limit, creating a new session may invalidate an older one.
- When the older session's next API call returns 401 and the refresh fails, the session expiration flow (6.1) triggers.

---

## 7. 403 Access Denied Page

When a user navigates to a URL they don't have permission to access (either via direct URL entry or a stale bookmark):

```
┌──────────────────────────────────────┐
│                                      │
│   (ShieldOff icon)                   │
│                                      │
│   Access Denied                      │
│                                      │
│   You don't have permission to       │
│   access this page.                  │
│                                      │
│   [Go to Dashboard]                  │
│                                      │
└──────────────────────────────────────┘
```

This page renders within the application shell (sidebar and top bar remain visible).
