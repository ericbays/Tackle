# 02 — The Threat Boundary & Evasion

## 1. The Threat Boundary
A critical architectural realization of the Tackle Framework is understanding the exact boundary of the Threat Model. 

Defensive teams and static analysis tools (Secure Web Gateways, Email Filters, Browser EDRs) **never** have access to the compiled Go binaries or the internal infrastructure. The infrastructure is entirely isolated. The only surface area exposed to defenders is the web-accessible HTTP response (HTML, React JS bundles, CSS, and structural flow) served through the Phishing Endpoint proxy during an active campaign.

## 2. Frontend Signature Evasion
Because the frontend is the only exposed surface, **Signature Evasion is entirely a Frontend and Traffic requirement**.
- The "offensive code" executing in the browser (the React logic, DOM structures, generated CSS, and telemetry collectors) cannot be static.
- Everything served to the frontend must be procedurally obfuscated and varied by the Compilation Engine to ensure no two campaigns share identical HTML structures, class naming conventions, or identifiable JavaScript footprints.

## 3. Backend Structural Consistency
Because the Go backend is invisible to the target and their defensive teams, it does **not** need to be procedurally fuzzed or randomized at the source-code layer.
- The Go backend logic that handles the heavy lifting (metric tracking, event captures, tracking token ingestion, and upstream forwarding to Tackle) can safely utilize standardized templates and reusable logic. 
- While the routes and API endpoints that the *frontend* reaches out to might be varied (e.g., dynamically naming a POST endpoint `/auth/v2` vs `/login/submit`), the underlying Go mechanisms executing the logic remain structurally consistent and secure.
