# 07 — Asset Management & File Serving

## 7.1 Overview

Landing applications require visual assets (images, logos, icons, fonts) and may need to serve files to targets (payload documents, downloads). The asset management system allows operators to upload files through the builder, which are then embedded directly into the compiled Go binary. At runtime, the binary serves these assets from memory — no filesystem access or external hosting required.

## 7.2 Asset Types

### Visual Assets

Assets used in the landing application's visual presentation.

| Asset Type | Use Case | Embedded As |
|-----------|----------|-------------|
| **Images** | Logos, backgrounds, photos, illustrations | `go:embed` in binary |
| **Icons** | UI icons, favicons | `go:embed` in binary |
| **Fonts** | Custom web fonts (woff2, woff, ttf) | `go:embed` in binary, served via CSS @font-face |
| **CSS Files** | Custom stylesheets | Bundled into the React CSS bundle |

### Payload Files

Files served to targets as part of the landing application's interaction flow.

| Asset Type | Use Case | Embedded As |
|-----------|----------|-------------|
| **Documents** | PDF, DOCX, XLSX, PPTX | `go:embed` in binary |
| **Archives** | ZIP, RAR | `go:embed` in binary |
| **Executables** | Tracking payloads, macro-enabled docs | `go:embed` in binary |
| **Other** | Any file the operator wants to serve | `go:embed` in binary |

## 7.3 Upload Flow

### Builder Upload Interface

Assets are uploaded through the builder UI. The upload interface is accessible from:

1. **Image/Logo component properties**: When editing an Image or Logo component, the `src` field provides an "Upload" button alongside the URL input
2. **Page favicon**: Upload a favicon in the Page Manager
3. **Asset Library panel**: A dedicated panel for managing all uploaded assets (accessible from the builder toolbar or left panel)
4. **File download configuration**: When configuring a download component or button to serve a file

### Upload Process

```
Operator selects file
        │
        ▼
┌─────────────────────┐
│  Client-side         │
│  validation          │  Size limit check, file type check
└────────────┬────────┘
             │
             ▼
┌─────────────────────┐
│  Upload to Tackle    │  POST /api/v1/landing-pages/{id}/assets
│  API                 │  Content-Type: multipart/form-data
└────────────┬────────┘
             │
             ▼
┌─────────────────────┐
│  Tackle stores       │  In database (as binary blob) or
│  asset               │  on filesystem with DB reference
└────────────┬────────┘
             │
             ▼
┌─────────────────────┐
│  Returns asset       │  Used in component properties
│  reference ID        │  (e.g., src="asset://abc123")
└─────────────────────┘
```

### Asset Reference Format

Uploaded assets are referenced in component properties using an internal URI scheme:

```
asset://{asset_id}
```

Example: An uploaded logo might have `src: "asset://a1b2c3d4"`

During compilation, the `servergen` pipeline resolves all `asset://` references to embedded file paths within the Go binary.

## 7.4 Asset Storage

### Database Storage

Each uploaded asset is stored with the following metadata:

```
Asset {
    id              : string       // UUID
    project_id      : string       // Which landing page project
    filename        : string       // Original filename
    content_type    : string       // MIME type (image/png, font/woff2, etc.)
    size_bytes      : integer      // File size
    data            : binary       // The file content (stored as bytea/blob)
    checksum        : string       // SHA-256 hash
    created_at      : datetime
}
```

### Size Limits

| Limit | Value |
|-------|-------|
| Single file upload | 10 MB |
| Total assets per project | 50 MB |
| Maximum number of assets per project | 200 |

## 7.5 Asset Embedding in Compiled Binary

During the `servergen` compilation pipeline:

1. All assets referenced by the project are fetched from the database
2. Assets are written to the build workspace directory under an `assets/` folder
3. A Go source file is generated with `//go:embed assets/*` directives
4. The Go binary embeds all assets at compile time
5. A Go handler serves embedded assets at deterministic paths (e.g., `/assets/{hash}.{ext}`)

### Generated Asset Routes

Each embedded asset is served at a path based on its content hash:

```
/assets/a1b2c3d4e5f6.png
/assets/f6e5d4c3b2a1.woff2
```

Component references in the generated HTML/React code point to these paths.

## 7.6 File Download Tracking

When the operator configures a button or link to serve a payload file to the target, the download is tracked.

### Configuration in Builder

The operator configures a download action on a Button or Link component:

```
Click Action: Serve File Download
  File: [Select uploaded file ▼]
  Download filename: "Q4-Report.pdf"     // What the target sees
  Track download: [✓]
  Mark as payload: [✓]                   // Flags for special attention in Tackle
```

### Download Flow

```
Target clicks download button/link
        │
        ▼
┌─────────────────────┐
│  Browser requests    │  GET /assets/download/{file_id}
│  file from landing   │
│  app                 │
└────────────┬────────┘
             │
             ▼
┌─────────────────────┐
│  Go handler          │
│                      │
│  1. Record download  │  POST download event to Tackle metrics
│     event            │
│  2. Set Content-     │  Content-Disposition: attachment;
│     Disposition      │  filename="Q4-Report.pdf"
│  3. Serve file       │  Stream embedded file bytes
│     from embed       │
└─────────────────────┘
```

### Download Event Data

```
DownloadEvent {
    file_id          : string       // Asset ID
    filename         : string       // Displayed filename
    is_payload       : boolean      // Whether marked as payload
    content_type     : string       // MIME type
    size_bytes       : integer      // File size
    source_ip        : string       // Target's IP
    user_agent       : string       // Target's browser
    timestamp        : datetime
    tracking_token   : string       // Per-target identifier
}
```

### Payload Tracking

When a file is marked as a payload:
- Tackle records the download with a `payload_delivered` event type (instead of generic `file_download`)
- If the payload is designed to phone home (e.g., a macro-enabled document that calls back), Tackle can correlate the callback with the download event via tracking identifiers embedded in the payload
- The operator can see payload delivery status in the campaign dashboard

## 7.7 Custom Fonts

### Upload

Operators upload font files (woff2, woff, ttf, otf) through the Asset Library.

### Usage

Font assets are made available in the builder's font-family dropdown and in the global styles editor. The `servergen` pipeline generates `@font-face` CSS rules for each uploaded font:

```css
@font-face {
    font-family: 'CustomFont';
    src: url('/assets/f6e5d4c3b2a1.woff2') format('woff2');
    font-weight: normal;
    font-style: normal;
}
```

## 7.8 Asset Library Panel

The builder provides an Asset Library panel for managing all uploaded assets:

```
┌──────────────────────────────────────┐
│ Asset Library                    [+] │
│                                      │
│ ┌────┐ ┌────┐ ┌────┐ ┌────┐        │
│ │ 🖼 │ │ 🖼 │ │ 📄 │ │ 🔤 │        │
│ │logo│ │bg  │ │Q4  │ │font│        │
│ │.png│ │.jpg│ │.pdf│ │.wf2│        │
│ └────┘ └────┘ └────┘ └────┘        │
│                                      │
│ Filter: [All ▼] [Images|Fonts|Files] │
│                                      │
│ Total: 4 assets (2.3 MB / 50 MB)    │
└──────────────────────────────────────┘
```

### Operations

- **Upload**: Add new assets (drag-and-drop or file picker)
- **Delete**: Remove unused assets
- **Preview**: View images, display font samples
- **Copy Reference**: Copy the `asset://` URI for use in properties
- **Rename**: Change the display name (original filename preserved)

## 7.9 Import Considerations

### HTML Import

When importing HTML (via file or URL clone), external asset references are handled:

- **Images**: Downloaded and uploaded as project assets (up to 2 MB each, base64 inlined if smaller)
- **CSS**: Inlined as `<style>` blocks
- **JavaScript**: Inlined as `<script>` blocks (if JS inclusion is enabled)
- **Fonts**: External font references are preserved as URLs (not downloaded)

### ZIP Import

When importing from a ZIP archive:

- All image files in the archive are uploaded as project assets
- CSS and JS files are inlined into the page definition
- Other files are uploaded as general assets
- Path traversal protection is enforced
