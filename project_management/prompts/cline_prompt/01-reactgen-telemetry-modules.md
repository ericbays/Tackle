# Qwen Task: Implement Malicious React Telemetry Hooks

## Context
We are completely overhauling the Tackle Landing Application Builder backend compiler. The Golang backend now transpiles a JSON AST structurally into a React application, bundles it using `esbuild`, and statically embeds the Javascript bundle into the final Go binary.

I (Lead Developer) have already built the core parser, CSS-in-JS generation, file I/O framework, and `esbuild` integration natively in Golang within the `internal/compiler/reactgen/` package. The transpilation engine is stable.

## Your Objective
You must implement the explicit offensive telemetry payloads in Go that dynamically inject malicious React components into the compiled Javascript bundle.

Inside `internal/compiler/reactgen/transpiler.go`, locate the two stubbed-out functions at the bottom of the file:
1. `func generateCampaignHooks(campaignType string) string`
2. `func generateCampaignHookActivators(campaignType string) string`

### 1. `generateCampaignHooks(campaignType string) string`
This function returns raw string literals representing declared React hooks that will sit outside the main export of `App.tsx`.
- **Benign OpSec:** All Javascript route names, variables, and function names MUST use completely benign, standard terminology. Do NOT use `telemetry`, `harvest`, `intercept`, or `fingerprint`. Use standard boilerplate words like `useSessionManager`, `syncState`, `/api/v1/auth/session`, etc.
- **Session Exfiltration:** If `campaignType == "basic_harvest"` or `advanced_proxy`, generate a hook that silently enumerates `document.cookie`, `localStorage`, and `sessionStorage`. 
- **Form Focus Listeners:** Generate a React hook that attaches `onFocus` listeners to standard input elements in the DOM. If triggered, it silently POSTs a status signal back to the server (tracking interaction intent without clicking submit).

### 2. `generateCampaignHookActivators(campaignType string) string`
This function returns string literals that are injected *inside* the `export default function App() {` block. 
- It must physically call the hooks you generated above using `useEffect` loops.
- It must start a passive "Time on Page" heartbeat interval that `fetch` pings a benign route (e.g., `POST /api/v1/ping`) every 30 seconds.
- If `campaignType == "awareness"`, only activate the generic ping heartbeat and do NOT activate any of the cookie exfiltration hooks. Only activate session exfiltration algorithms if the `CampaignType` allows for it!

## Rules for Success
1. **Focus Only on Go Strings:** You are writing Golang code that outputs React Typescript strings. Ensure your returned strings are perfectly formatted valid React syntax (escaping quotes properly in Go is crucial). 
2. **Do Not Touch Escaping Architecture:** Do not rip out my `AST` or `Node` structs. Limit your changes explicitly to rewriting those two stub functions at the bottom of the file.
3. Test your formatting before yielding. Ensure the Go file compiles and the string outputs are clean.
