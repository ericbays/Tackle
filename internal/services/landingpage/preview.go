package landingpage

import (
	"encoding/json"
	"fmt"
	"html"

	"tackle/internal/compiler/reactgen"
)

// RenderPreviewHTML generates an HTML preview from a page definition.
// This integrates natively with the reactgen esbuild transpilation pipeline.
func RenderPreviewHTML(def map[string]any, pageIndex int) (string, error) {
	pages, ok := def["pages"].([]any)
	if !ok || len(pages) == 0 {
		return "", fmt.Errorf("no pages in definition")
	}

	if pageIndex < 0 || pageIndex >= len(pages) {
		pageIndex = 0
	}

	page, ok := pages[pageIndex].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid page at index %d", pageIndex)
	}

	title, _ := page["title"].(string)
	if title == "" {
		title = "Preview"
	}
	favicon, _ := page["favicon"].(string)

	astPayload := map[string]any{
		"campaignType": "awareness", // Lock previews to safe rendering tracking-only
	}

	var componentNodes []any
	if tree, ok := page["component_tree"].([]any); ok {
		componentNodes = tree
	}

	astPayload["rootNode"] = map[string]any{
		"id":   "builder-root-container",
		"type": "root",
		"styles": map[string]string{
			"width": "100%",
			"minHeight": "100vh",
			"display": "flex",
			"flexDirection": "column",
			"margin": "0",
			"padding": "0",
		},
		"children": componentNodes,
	}

	b, err := json.Marshal(astPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal component tree: %w", err)
	}

	appJsx, err := reactgen.Transpile(b)
	if err != nil {
		return "", fmt.Errorf("reactgen transpile failed: %w", err)
	}

	ws, err := reactgen.CreateWorkspace()
	if err != nil {
		return "", fmt.Errorf("workspace creation failed: %w", err)
	}
	defer ws.Cleanup()

	if err := ws.WriteIndex(); err != nil {
		return "", fmt.Errorf("write index failed: %w", err)
	}

	if err := ws.WriteFile("App.tsx", appJsx); err != nil {
		return "", fmt.Errorf("write App.tsx failed: %w", err)
	}

	res, err := reactgen.RunEsbuild(ws.DirPath)
	if err != nil {
		return "", fmt.Errorf("esbuild compilation failed: %w", err)
	}

	var faviconTag string
	if favicon != "" {
		faviconTag = fmt.Sprintf(`    <link rel="icon" href="%s">`, html.EscapeString(favicon))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
%s
    <style>body{margin:0;padding:0;box-sizing:border-box;font-family:sans-serif;}*,*:before,*:after{box-sizing:inherit;}</style>
    <style>%s</style>
	<!-- ESM Import map to bypass node_modules locally -->
	<script type="importmap">
	{
		"imports": {
			"react": "https://esm.sh/react@18",
			"react-dom/client": "https://esm.sh/react-dom@18/client"
		}
	}
	</script>
</head>
<body style="margin: 0; padding: 0; overflow-x: hidden;">
    <div style="position:fixed;top:0;left:0;right:0;background:#ff6b00;color:#fff;text-align:center;padding:4px 0;font-family:sans-serif;font-size:12px;font-weight:bold;z-index:99999;">PREVIEW MODE (REACT COMPILED)</div>
	<div id="root"></div>

    <script type="module">%s</script>
</body>
</html>`, html.EscapeString(title), faviconTag, string(res.CSS), string(res.JS)), nil
}
