# 4. Refactoring Anti-Fingerprinting for React

## 4.1 The Challenge
The original anti-fingerprinting system (`internal/compiler/randomizer/`) was explicitly written for string-based HTML parsing. It searches for `<div>` blocks using regex/AST and injects arbitrary attributes or decoy sibling elements.

In a React transpilation environment, the physical HTML does not exist during generation—it is generated entirely client-side by ReactDOM. Therefore, evasion and randomization mechanics must be migrated into the React generation layer.

## 4.2 Dynamic Class Name Shuffling (CSS Modules)
Rather than traversing a raw HTML string to find and replace `"class="row"`, the new React pipeline resolves this cleanly natively via CSS-in-JS abstractions or generated CSS Modules.

1. `reactgen` creates a deterministic map pairing logical roles to randomized evasion names (Seed `A`: `row` -> `.axqfm`, Seed `B`: `row` -> `.el-a7f3`).
2. When parsing the AST, the React components apply styles safely referencing the mapped hash:
```typescript
// Dynamically generated based on the Seed hash
const antiFingerprintMap = {
    row: "axqfm_kRtPx",
    input: "v2_sK29x",
};

export function CampaignRow({ children }) {
    return <div className={antiFingerprintMap.row}>{children}</div>
}
```

## 4.3 Structural Decoy Injection (DOM Variation)
To evade detection tools that map exact DOM shapes, structural complexity must be varied mathematically per build.

To accomplish this in React, the transpiler injects randomized conditional wrappers into the TSX output prior to the `.js` bundle execution.

```typescript
// Randomly decided at build time by the Go evasion engine based on the Seed.
const ENABLE_DECOY_WRAPPER_LEVEL_1 = true;

export function FormContainer({ children }) {
   // Wraps the critical content in plausible but mathematically useless nodes
   if (ENABLE_DECOY_WRAPPER_LEVEL_1) {
       return (
           <article className="af-container-42">
               <span aria-hidden="true" style={{display: 'none'}}>Account Verification</span>
               {children}
           </article>
       )
   }
   return <>{children}</>;
}
```

## 4.4 Header and Asset Path Randomization
These elements are independent of the React frontend rewrite and can be retained largely unchanged.

- **Assets:** Because the Go native `esbuild` wrapper generates the `styles.css` and `bundle.js`, the Go `asset_randomizer.go` can safely intercept the files in memory before they are injected into the binary, hashing their filenames precisely as it does currently.
- **Headers:** `header_randomizer.go` is implemented as standard Golang HTTP middleware wrapping the Go webserver, completely separated from the HTML/React engine logic. This component requires zero adjustments.
