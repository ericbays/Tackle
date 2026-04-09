# 1. Architectural State & Disconnect

## 1.1 The Ultimate Objective
The core requirement defined in the Tackle documentation (`05-landing-page-builder.md`, Section 1) explicitly dictates that the framework must compile the builder's JSON AST into a **standalone React frontend + Go backend binary**. This architecture guarantees modern rendering, robust state management, and parity with standard web application development.

## 1.2 The Current Implementation (The Disconnect)
An analysis of the existing codebase reveals that the current compilation engine entirely ignores the React frontend requirement. Currently, the Golang deployment compiler strictly outputs **static, vanilla HTML strings**.

### Technical Tracing of the Error
1. **The Entry Point:** Compilation begins in `internal/compiler/engine.go` (`runBuild` method).
2. **The Code Generator:** The step that processes the JSON AST relies on `internal/compiler/htmlgen/generator.go` (specifically `GeneratePageAssets`).
3. **The Rendering Mechanism:** Inside `renderProductionComponent`, the Golang code iterates through the structural nodes of the JSON tree. Instead of transpiling this data into React components or JSX, it forcefully concatenates standard HTML strings:
   ```go
   case "heading":
       w.WriteString("<h2>" + content + "</h2>")
   case "row":
       w.WriteString("<div style='display:flex'>" ... "</div>")
   ```
4. **The Bundler:** The process is finalized via `gogen.GenerateGoSource()`, which creates a Go HTTP server that serves the raw HTML text blobs in a `static/` directory.

### 1.3 The Root Cause
This architectural split originated in the prompt templates utilized by the previous AI developers (Claude + Qwen 3). Specifically, the anti-fingerprinting directives (Section 4.3, Tasks AF-1 through AF-6) rigidly ordered the creation of a "HTML DOM Randomization Engine." 

Because standard static HTML is significantly easier to inject with decoys, regex, and random attributes using native Golang libraries than a bundled React application, the isolated Qwen implementation abandoned React entirely and built a primitive string generator.

## 1.4 The Impact
- **Severe Frontend Friction:** The Admin Landing Application Builder is built fundamentally using a React paradigm (relying on complex component states and modern Flexbox implementations). Attempting to map advanced React capabilities directly onto 1990s-style static HTML tags creates constant rendering conflicts, CSS specificity wars, and broken `Preview` environments.
- **Incompatible HMR (Hot Reloading):** True Hot Module Replacement (HMR) and real-time development rendering are mathematically impossible when relying on static string-concatenated HTML injected via `document.write()`.

## 1.5 Goal of the Overhaul
To correct this, the compilation package `internal/compiler/htmlgen` must be deprecated and systematically replaced with a transpilation engine that accurately converts the JSON AST into compiled React source code, thereby fulfilling the original architectural vision while retaining full anti-fingerprinting capabilities.
