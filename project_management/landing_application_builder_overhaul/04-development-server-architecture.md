# 04 — Development Server Architecture

## 1. The Mode Split (Dev vs. Prod)
Because the Production Compilation relies on procedural obfuscation and standalone binary embedding to achieve Signature Evasion, it is impossible to run sequentially on every component drag-and-drop within the builder UI. Doing so would take too long and destroy the operator's real-time experience.

To solve this, Tackle splits generation into two specific modes:
1. **Production Mode:** Procedurally fuzzed Go source, randomized React bundle, statically embedded via `//go:embed`, running on a single port.
2. **Development Mode:** Un-obfuscated baseline templating executed dynamically, utilizing a dual-port bridging architecture to support Hot Module Replacement (HMR).

## 2. Dual-Port Architecture
When the operator clicks "Deploy Development Server" or previews the Builder Canvas, Tackle compiles a lightweight proxy version of the Landing Application.

This Development Application requires two distinct network ports:
- **The Application Backend (Port A):** The Go binary binds to the first port to handle all internal API traffic, state tracking, and capture routing.
- **The React Dev Server (Port B):** The Go binary programmatically spawns a native React development process (e.g., Vite/esbuild dev server) on a second port. This enables true, real-time HMR.

## 3. Autonomous Self-Registration (Bottom-Up)
Tackle itself does not control port allocation from the top down, and it does not parse OS processes to clear "zombies." The Development Application is entirely autonomous.
1. Tackle compiles and executes the Dev binary.
2. The child binary dynamically requests two open ports from the OS.
3. Once booted, the child fires a webhook back to Tackle (`POST /api/internal/dev-server/register`) providing its assigned Frontend and Backend ports.
4. **Heartbeat Lifecycle:** The child application begins emitting a steady heartbeat to Tackle. If the connection fails or Tackle goes offline, the Development Application cleanly terminates itself (`os.Exit(0)`), completely eliminating orphaned zombie processes.

## 4. HMR Data Flow
1. Operator moves a component in the Tackle UI.
2. Tackle `POST`s the updated JSON AST to the child Backend's HMR ingress route.
3. The child Backend broadcasts the AST directly to its own React frontend via a local WebSocket (`/dev-hmr/ws`).
4. The React context state updates instantly, avoiding a full page refresh.
