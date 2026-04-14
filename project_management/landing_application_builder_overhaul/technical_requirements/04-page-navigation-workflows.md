# 04 — Page Navigation & Workflow Engine

## 4.1 Overview

The Workflow Engine is the system by which operators define how targets move through a multi-page landing application. Operators configure event-driven, conditional navigation flows that determine what happens when a target submits a form, clicks a button, or lingers on a page. The entire flow — from the first page a target sees to the final redirect — is defined visually in the builder's Workflow Editor panel.

## 4.2 Core Concepts

### Pages as States

Each page in the landing application represents a state in the application flow. The target is always on exactly one page at a time. Navigation between pages is triggered by events.

### Events as Transitions

Transitions between pages are triggered by one of the following event types:

| Event Type | Description | Example |
|------------|-------------|---------|
| **form_submit** | A form on the current page is submitted | Target submits login credentials |
| **click** | A specific element is clicked | Target clicks a "Continue" button |
| **timer** | A countdown expires after the page loads | Show loading screen for 3 seconds, then redirect |
| **page_load** | The page finishes loading | Auto-redirect on load (e.g., an intermediate redirect page) |

### Conditional Routing

Transitions can be conditional. The operator defines rules that evaluate the current state to determine which page to navigate to next.

## 4.3 Workflow Definition Structure

Each workflow rule (called a **Flow**) is defined by the operator in the Workflow Editor:

```
Flow {
    id              : string       // Unique flow identifier
    source_page     : string       // Page ID or route where the flow originates
    trigger         : EventType    // form_submit, click, timer, page_load
    trigger_target  : string       // (Optional) DOM ID of the triggering element
    conditions      : array        // (Optional) Conditional routing rules
    default_target  : string       // Target page route (if no conditions match)
    delay_ms        : number       // (Optional) Delay before navigation (ms)
}
```

### Condition Structure

```
Condition {
    field           : string       // Form field name to evaluate
    operator        : string       // equals, not_equals, contains, exists, not_exists
    value           : string       // Expected value
    target_page     : string       // Page route to navigate to if condition is true
}
```

## 4.4 Event Types in Detail

### 4.4.1 Form Submit

Triggered when a `<form>` on the source page is submitted.

**Trigger target**: The DOM ID of the specific form. If omitted, the flow applies to any form submission on the page.

**Behavior sequence**:
1. Target fills in form fields and clicks submit
2. The form POSTs to the operator-configured action path (e.g., `/signin`)
3. The Go backend captures all form field data
4. The Go backend forwards captured data to Tackle's internal API
5. The Go backend fires a `form_submission` metric event
6. The Go backend evaluates the flow's conditions against the submitted data
7. If conditions match → navigate to the condition's target page
8. If no conditions match → navigate to the default target page
9. Navigation respects the configured delay (if any)

**Example — Login with conditional MFA**:
```
Flow:
  source_page: /signin
  trigger: form_submit
  trigger_target: login-form
  conditions:
    - field: "email"
      operator: contains
      value: "@contoso.com"
      target_page: /mfa
  default_target: /dashboard
  delay_ms: 0
```

This routes `@contoso.com` users to an MFA page, and all others directly to a dashboard.

### 4.4.2 Click

Triggered when a specific element on the page is clicked.

**Trigger target**: The DOM ID of the clickable element (required for click triggers).

**Example — Continue button routing**:
```
Flow:
  source_page: /consent
  trigger: click
  trigger_target: continue-btn
  default_target: /download
  delay_ms: 0
```

### 4.4.3 Timer

Triggered automatically after a specified delay when the page loads.

**Delay_ms**: Required. The number of milliseconds to wait before navigating.

**Example — Loading page with auto-redirect**:
```
Flow:
  source_page: /loading
  trigger: timer
  default_target: /dashboard
  delay_ms: 3000
```

This shows a loading/spinner page for 3 seconds, then redirects.

### 4.4.4 Page Load

Triggered immediately when the page finishes rendering. Useful for immediate redirects or initialization.

**Example — Intermediate redirect**:
```
Flow:
  source_page: /redirect
  trigger: page_load
  default_target: /final
  delay_ms: 0
```

## 4.5 Condition Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `equals` | Exact string match | `field: "domain", value: "contoso.com"` |
| `not_equals` | Does not match | `field: "role", value: "admin"` |
| `contains` | Substring match | `field: "email", value: "@contoso"` |
| `exists` | Field is present and non-empty | `field: "mfa_code"` |
| `not_exists` | Field is absent or empty | `field: "mfa_code"` |

### Condition Evaluation Order

Conditions are evaluated in the order they are defined. The first matching condition wins. If no conditions match, the `default_target` is used. If no `default_target` is defined, the target stays on the current page.

## 4.6 Workflow Editor UI

The Workflow Editor is accessed via the "W" tab in the builder's left panel.

### Flow List

Displays all defined flows for the current project, grouped by source page.

### Flow Editor Form

When adding or editing a flow:

| Field | Control | Notes |
|-------|---------|-------|
| Source Page | Dropdown (all pages) | Which page this flow originates from |
| Trigger | Dropdown (form_submit, click, timer, page_load) | What event triggers the flow |
| Trigger Target DOM ID | Text input | (form_submit, click only) DOM ID of the triggering element |
| Conditions | Dynamic list | Add/remove conditional routing rules |
| Default Target | Dropdown (all pages) + "External URL" option | Where to navigate if no conditions match |
| Delay (ms) | Number input | Optional delay before navigation |

### Condition Row

Each condition in the list:

| Field | Control |
|-------|---------|
| Form Field Name | Text input (name of the form field to check) |
| Operator | Dropdown (equals, not_equals, contains, exists, not_exists) |
| Value | Text input (expected value; hidden for exists/not_exists) |
| Target Page | Dropdown (all pages) + "External URL" option |

### Flow Operations

- **Add Flow**: Creates a new empty flow
- **Edit Flow**: Opens the flow editor form
- **Delete Flow**: Removes the flow (with confirmation)
- **Reorder Flows**: Drag to change evaluation priority (multiple flows on same page/trigger)

## 4.7 Navigation Execution in the Compiled Application

### Client-Side Navigation

For internal page transitions (target stays within the landing application), navigation is handled client-side by the React application. The compiled React app includes a router that:

1. Listens for form submissions and click events on elements with matching DOM IDs
2. Evaluates conditions against the current state
3. Performs client-side navigation to the target page route
4. Updates the browser URL bar (using history.pushState)

### Server-Side Navigation

For form submissions, the Go backend handles the POST and returns the navigation instruction:

1. Form POSTs to the operator-defined action path
2. Go handler captures data, forwards to Tackle
3. Go handler evaluates flow conditions against submitted form data
4. Go handler returns a response that tells the React frontend which page to navigate to
5. React frontend navigates to the target page

This ensures that form data is captured by the backend before any navigation occurs.

### External Redirects

When the target page is an external URL:
1. The Go backend (for form_submit) or React frontend (for click/timer/page_load) performs a full browser redirect via `window.location.href`
2. The target leaves the landing application entirely

## 4.8 Flow Validation

The builder validates flows and warns the operator about:

- **Unreachable pages**: Pages that have no incoming flows (except the entry page)
- **Dead ends**: Pages with no outgoing flows and no external redirects
- **Missing trigger targets**: Click/form_submit flows referencing DOM IDs that don't exist on the source page
- **Circular flows**: A → B → A without any user interaction (infinite loops via page_load/timer)
- **Orphaned flows**: Flows referencing pages that have been deleted

Validation warnings are non-blocking — the operator can still save and build with warnings.

## 4.9 Multi-Flow Scenarios

### Multiple Flows per Page

A single page can have multiple flows. For example, a page with both a form and a "Skip" button:

```
Flow 1:
  source_page: /mfa
  trigger: form_submit
  trigger_target: mfa-form
  default_target: /dashboard

Flow 2:
  source_page: /mfa
  trigger: click
  trigger_target: skip-btn
  default_target: /dashboard
```

### Flow Priority

When multiple flows could match the same event (e.g., two form_submit flows on the same page), they are evaluated in order. The first matching flow is executed.

## 4.10 Example: Complete Application Flow

**Scenario**: Microsoft 365 credential harvester with conditional MFA

```
Pages:
  /signin     - Login form (email + password)
  /mfa        - MFA code entry
  /loading    - Loading spinner
  /success    - "You're all set" message

Flows:
  1. source: /signin, trigger: form_submit, target: mfa-form
     conditions:
       - field: email, operator: contains, value: "@corp.com" → /mfa
     default_target: /loading

  2. source: /mfa, trigger: form_submit, trigger_target: mfa-form
     default_target: /loading

  3. source: /loading, trigger: timer, delay_ms: 2500
     default_target: /success

  4. source: /success, trigger: timer, delay_ms: 5000
     default_target: https://office.com (external redirect)
```

**Target experience**:
1. Target lands on `/signin`, enters email and password, clicks "Sign In"
2. If email contains `@corp.com` → goes to MFA page; otherwise → loading page
3. Target enters MFA code (if applicable), submits → loading page
4. Loading spinner shows for 2.5 seconds → success page
5. Success message shows for 5 seconds → redirected to real Office 365

**What Tackle captures**: Email, password, MFA code (if entered), plus page view and form submission events at each step.
