# 03 — Go Backend Procedural Generation

## 1. The Dynamic Requirement

A massive architectural flaw in initial planning was assuming the generated Go backend acted as a "simple, static API." It does not. 

The Landing Application Builder allows Red Team operators to build highly specialized logic:
- A 1-page credential harvester.
- A 15-page recursive application requiring dynamic state management.
- Conditional multi-page workflows (e.g., exposing page 4 only if specific conditions are met on page 3).

Because the React frontend can be mapped in various anti-fingerprinting permutations (SPA, Multi-File, or Hybrid), the underlying Go backend must be completely procedurally generated.

## 2. No Static Handlers
Tackle does **not** rely on a static `routes.go` or pre-compiled backend handlers for processing target interactions.

Instead, the **Compilation Engine** (specifically the `servergen` module) translates the operator's JSON builder logic directly into procedural Go source code.
- If the operator defines 15 pages that use Server-Side routing, `servergen` dynamically writes 15 unique Go `http.HandlerFunc` functions.
- If the operator defines conditional logic (like exposing a page under certain circumstances), `servergen` writes the equivalent Go conditional routing logic into the Target Application's source code before it compiles.
- Every form submission endpoint, asset route, and API ingress path is uniquely randomized and explicitly written into the target's `main.go` logic.

## 3. The Compilation Workflow (Engineering Directives)

To achieve this procedural generation, Tackle's compilation process operates as follows:
1. **Workspace Generation:** Tackle creates an isolated temporary directory for the campaign build.
2. **Dynamic Templating (`servergen`):** Tackle uses Go's `text/template` engine—not to drop in static files, but to dynamically write the pure Go source code required to satisfy the complex routing, conditional logic, and telemetry hooks defined by the operator.
3. **Asset Embedding:** Once the Frontend obfuscator finishes generating the UI files (React builds, CSS, JavaScript decoys), they are placed directly into the workspace. The generated Go code uses the standard compiler injection (`//go:embed`) to fuse these files directly into the Go executable natively.
4. **Binary Compilation:** Tackle executes `go build` against the temporary workspace, yielding a single, perfectly custom Target Application binary. The workspace is then destroyed.
