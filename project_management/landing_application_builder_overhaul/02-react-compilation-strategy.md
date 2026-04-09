# 2. React Compilation Strategy Overhaul

## 2.1 Concept
To achieve the requirement of a standalone React + Golang binary, the Go backend cannot rely on basic HTML string generation. Instead, Go must dynamically write React source code to a temporary build directory, bundle it into an optimized payload, and embed that payload into the compiled Go HTTP server.

## 2.2 Transpilation: AST to JSX
The new compiler engine (e.g., `internal/compiler/reactgen`) will parse the builder's JSON component tree and dynamically generate physical `.tsx` files.

Instead of writing string literals (`<div class="row">`), the compiler outputs standard React abstractions.
**Generated `App.tsx` example:**
```typescript
import { CampaignRow, CampaignColumn, CampaignButton } from './components';
import './compiled.css';

export default function LandingApplication() {
  return (
    <CampaignRow data-af-id="q24zx">
      <CampaignColumn width="full">
        <CampaignButton onClick={captureTelemetry}>Submit</CampaignButton>
      </CampaignColumn>
    </CampaignRow>
  );
}
```

## 2.3 The Bundling Pipeline (Native Go Esbuild)
Relying on system dependencies like Node.js or `npm run build` during server-side compilation is brittle and extremely slow, taking 5-15 seconds.

Instead, Tackle should utilize **esbuild** natively via its Go API wrapper (`github.com/evanw/esbuild/pkg/api`). Esbuild is written in Go and can process, bundle, and minify a full React application in **less than 100 milliseconds** internally without needing Node installed on the deployment machine.

### Flow Architecture:
1. `reactgen` creates a temp directory in `/tmp/tackle-build-<uuid>`.
2. Go iterates over the JSON AST and writes React TSX files representing the structural layout.
3. Go invokes the native `esbuild` library to bundle `App.tsx` and generating the compiled anti-fingerprinted `styles.css`.
4. **Cleanup**: Go deletes the temporary TSX source code files, leaving only the executed `bundle.js`, `styles.css`, and a shell `index.html`. (Browsers cannot read TSX; once the JS bundle is generated, the source files are obsolete overhead).
5. **Embedding**: `gogen` wraps these static bundled files into the Go codebase using the `embed` directive (`//go:embed static/*`).
6. `go build` runs, generating an incredibly fast, highly optimized standalone binary capable of serving the React app.

## 2.4 Managing State and Telemetry
By outputting a true React application, state management becomes trivialized.
Form capture logic, keylogging telemetry, and mouse tracking (dictated by the anti-fingerprinting behavior profile) can be managed via a global context provider (`<TelemetryProvider>`) wrapping the generated React application, seamlessly forwarding data to the Go backend endpoints seamlessly using modern `fetch` APIs.
