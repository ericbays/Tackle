# 3. Dev Server ("Deploy Development") Mechanics

## 3.1 Resolving the Preview Deficiencies
The prior `/preview` endpoint rendered a static HTML string injected into an `about:blank` window, causing severe relative-path issues, CORS blocks, and CSS conflicts. 

The replacement is a dedicated remote Development Server. When an operator clicks "Deploy Development", the Tackle backend natively boots a temporary instance of the generated React + Golang project on a live port. 

> **CRITICAL REQUIREMENT:** The Development build and the Production build must be structurally and mathematically identical. The only deviation is that the Development build injects a `DevModeProvider` wrapper for real-time WebSocket HMR. When a project is compiled for Production, this wrapper is completely stripped, ensuring the operator experiences the precise 1:1 final product during development.

## 3.2 Dynamic Port Allocation and Process Lifecycle
To ensure zero port collisions, the Golang backend must implement a `DevServerManager`.

1. **Port Sniffing**: Go leverages its `net` standard library to test for available TCP ports in a high range (e.g., `net.Listen("tcp", "127.0.0.1:0")` mathematically guarantees an open, non-colliding port).
2. **Process Spawning**: Go executes the compiled binary utilizing `os/exec.Command`.
3. **PID Tracking & Zombie Cleanup**: The `DevServerManager` maps the `ProjectID` -> `PID`. A background garbage collection goroutine periodically pings the Dev servers. If the Admin user's session terminates or the Builder unmounts, the Dev Server process is explicitly killed via `syscall.SIGTERM`. Ensures only one Dev Server per project is active.

## 3.3 True Real-Time Updates (HMR Context Engine)
Creating a fast feedback loop is paramount. Waiting for Go to recompile the binary (even 2 seconds) on every margin adjustment creates frustrating UX.

Since the final application is now React-based `(see 02-react-compilation-strategy.md)`, we can implement real-time layout updates natively without restarting the Go server.

### Architecture:
1. **The Dev Wrapper**: During Development Compile Mode, the generated React application is wrapped in a `<DevModeProvider>`. This provider establishes a WebSocket connection back to the **Tackle Admin Server**.
2. **Dynamic Rendering**: Instead of hardcoding the React JSX into static files during development, the `DevModeProvider` accepts the JSON AST dynamically over the socket.
3. **The Loop**:
   - The user drags a component in the Admin Builder.
   - The Admin Builder pushes the updated JSON AST over the WebSocket.
   - The `DevModeProvider` in the Live Dev Server receives the payload, drops it into React state, and triggers a re-render.
   - The React engine instantly redraws the live application mimicking Hot Module Replacement (HMR).

### Hard Restarts:
A hard restart (terminating the PID and recompiling) is only triggered when modifying backend constraints (e.g., changing the credential capture URL logic, modifying DNS redirects, or enabling a new core plugin module) rather than structural UI adjustments.
