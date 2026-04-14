# 05 — Telemetry & Intercept Routing

## 1. Dynamic Intercept Handling
The Landing Application is capable of handling any scenario (Credential Capture, File Downloads, WebApp Simulations). The Operator dynamically constructs these capabilities within the Builder UI. 

When configuring interactions (e.g., a "Sign In" form), the Operator defines:
1. **The Ingress Route:** The exact URL path the Target's browser will POST to (e.g., `/signin`, `/api/v1/auth`, `/submit`).
2. **The Expected Data Structure:** What fields the handler should anticipate.
3. **The Exit Strategy:** What the application should do after receiving the payload (e.g., redirect the user to a legitimate portal, serve a 200 OK, return a mock JSON error).

The Target's browser sees a completely normal, legitimate transaction to an expected endpoint. Defense tools see the exact same benign HTTP traffic.

## 2. Behind The Scenes: The Upstream Pipeline
Once the dynamically generated Landing Application Backend receives the Target's payload at that custom route, it executes its internal requirements entirely out of view of the target:

1. It packages the raw loot (if credentials or data were submitted).
2. It packages the interaction metrics (e.g., "Form Submission Completed", Timestamp, IP context).
3. Using an internal HTTP client, the Landing Application Backend fires a standard REST POST upstream to the primary Tackle server (e.g., `/api/v1/internal/captures` and `/api/v1/internal/metrics`).

## 3. Network Isolation Boundary
This upstream communication happens **exclusively over the local inter-server network** (Tackle and the Landing Apps are hosted on the same internal server ecosystem within the private lab).

Tackle itself is completely firewalled from the Internet. The Phishing Endpoints (acting as transparent proxies on internet-accessible cloud instances) are the only entities that bridge public traffic down into the private lab. 

Because the Landing Application never routes its internal telemetry out to the open web or across the Phishing Endpoint proxy, defensive traffic inspection systems can never observe the existence of the Tackle metrics pipeline. To the outside world, the Landing Application appears to "consume" the credentials entirely on its own.
