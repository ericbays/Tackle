# 6. Advanced Capture & Metrics Integration

## 6.1 Overview
The architectural overhaul must account for the extensive tracking and credential exfiltration requirements defined in `08-credential-capture.md` and `10-metrics-reporting.md`. The React frontend and the Go proxy backend must operate in tandem to fulfill these requirements securely without tipping off defensive heuristics.

## 6.2 Wizard-Driven AST Compilation
To manage the complexity of enterprise phishing logic, the Tackle operator's Frontend Builder utilizes a Setup Wizard prior to opening the canvas. This prevents operators from misconfiguring advanced routing rules when building simple payloads. 

From a backend perspective, this wizard fundamentally dictates the compilation pipeline. The generated JSON AST will define a root-level `CampaignType` (e.g., `awareness`, `basic_harvest`, `advanced_proxy`). 

The Golang compiler **must** use this root `CampaignType` as its primary discriminator:
- If `type: awareness`, the Go compiler intentionally ignores any deeply nested forms or session-capture toggles in the AST (preventing accidental misconfigurations) and outputs a mathematically gutted, lightweight React application capable only of Server-Side Link Tracking and basic telemetry heartbeats.
- If `type: advanced_proxy`, the Go compiler activates the full suite of React/Go TLS generation libraries necessary for Credential Replay modules.

## 6.3 React Frontend Telemetry Layer
The React compiler must bake in robust, stealthy event listeners that fulfill the metrics pipeline without polluting the network tab with obvious tracker names.

### Toggleable Modules & Built-in Interaction Beacons (REQ-MET-015, REQ-MET-016)
- **Conditional Compilation:** Advanced telemetry gathering is not hardcoded. The React compiler dynamically injects these modules into the final output only if the Tackle operator explicitly enables them in the Landing Application Builder configurations.
- **Form Focus Tracking & Heartbeats:** If enabled, the component tree includes a stealth heartbeat loop. If the user focuses on an input field or stays on the page, the React app quietly notifies the Go proxy.
- **Fully Configurable Routing:** Exactly like the credential POST routes, the telemetry beacon routes are fully configurable by the Tackle operator. The compiler does not enforce standard routes (like `/track`). The operator can configure the telemetry to hit `/api/v1/session/renew` for one campaign, and `/status` for another, ensuring total entropy.

### Conditional Session Exfiltration (REQ-CRED-021)
- If the Tackle operator toggles Session Capture "ON" for a specific campaign, the React generator injects a specialized payload. This payload enumerates `document.cookie`, `localStorage`, and `sessionStorage`. 
- This session data is packaged alongside the credential form POST, allowing Tackle to harvest OAuth tokens alongside plaintext passwords. If the operator leaves this off, the Javascript payload remains fundamentally lighter and simpler.

## 6.3 Advanced Golang Backend Replay & Proxy
The generated Go backend serves as much more than just a host for the React files; it is a critical Active Intercept proxy.

### Toggleable Credential Replay (REQ-CRED-018)
If the Tackle operator explicitly configures and enables "Replay submission" for a credential-harvesting campaign, the Go backend is compiled as an Active Intercept proxy.
1. The target submits the React form to the custom, benign local Go backend route configured by the operator.
2. The Go backend captures the credentials securely.
3. The Go backend establishes a TLS request to the *actual* service the campaign is mimicking (e.g., `login.microsoft.com`), passes the captured credentials, and receives the legitimate session cookies.
4. The Go backend relays those real session cookies back to the target's React browser instance, logging them in seamlessly.

If this module is toggled off by the operator (e.g. for a simple awareness campaign that doesn't capture passwords), the Go compiler omits this complex proxying layer entirely, resulting in a significantly lighter binary.

## 6.4 Handling Unattributed Tracking
If a blue team scanner or a curious user navigates to the phishing landing page stripped of its unique tracking parameters, the React frontend and Go backend must still function flawlessly. The architecture must gracefully fall back to "Unattributed Capture" (REQ-CRED-020), ensuring that security analysts investigating the page cannot break the app by messing with the URL state.
