# 5. Stealth Backend Telemetry Pipeline

## 5.1 The Telemetry Challenge
Landing pages must securely capture credentials, intercept cookies via reverse proxying, track email opens, log telemetry, and record user interactions without exposing the primary Tackle infrastructure to the target or defensive tooling.

If the landing page was strictly a static HTML/JS payload, the Javascript traversing the target's browser would be forced to communicate directly with the Tackle infrastructure, exposing API endpoints, Campaign Tokens, and network routes to anyone inspecting the browser's Network tab.

## 5.2 The React + Go Proxy Architecture
By compiling a standalone **React frontend paired with a locally generated Go backend**, the architecture seamlessly masks all upstream telemetry while appearing entirely benign to automated scanners.

### Operational Security (OpSec) Naming Constraints
A critical rule for the generated React Application is that it must **never** contain code, API routes, or variable names that imply malicious intercepting logic. Words like `intercept`, `fingerprint`, `capture`, or `telemetry` will immediately trigger defensive security tools. The frontend must look identical to a generic SaaS application.

### The Pipeline Mechanics:
1. **Server-Side Visit Tracking (No JS Required)**: When a target clicks the phishing link, they send a standard `GET /` request containing their unique identifier via URL parameters. The compiled Go backend natively parses this ID, securely POSTs the "Landing Link Clicked" event back to the main Tackle database, and *then* serves the React app. The React frontend is completely unaware this tracking occurred.
2. **Dynamic Operator-Controlled Routing**: The Tackle framework does not force standardized routes. The Tackle operator has full control over configuring the exact path the frontend form posts to (e.g., `/signin`, `/login.php`, or `/widufhwiufoiw`) directly inside the Landing Application Builder. This ensures maximum entropy across campaigns and allows the app to perfectly mimic the upstream target.
3. **The Silent Upstream Post**: The generated Go backend receives the localized `POST` payload at whatever custom route the operator configured. It intercepts the plaintext credentials, securely wraps them alongside the campaign's `BuildToken`, and establishes a secure server-to-server connection back to the root Tackle Central Database. 

## 5.3 Security & Evasion Implications
- **Zero Client Exposure**: The target's browser never sees the IP or domain of the actual Tackle Framework. The browser exclusively talks to the phishing endpoint.
- **Benign Surface Analysis**: Security appliances analyzing the React source code will only find standard local fetch requests (e.g., `fetch('/api/v1/auth')`) mirroring typical frontend application behavior.
- **Defensive Blindness**: The complex upstream telemetry logic, session hijacking mechanisms, and campaign identification are securely locked inside the compiled Go binary where no client-side scanner can inspect them.
