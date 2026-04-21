package servergen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GenerateReactApp parses the project definition JSON and generates the React source files
func GenerateReactApp(definition map[string]any, isDevelopment bool) (map[string]string, error) {
	files := make(map[string]string)

	// index.html - Template Shell
	files["index.html"] = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Landing Application</title>
    <link rel="stylesheet" href="/assets/index.css">
    <script src="https://unpkg.com/react@18/umd/react.production.min.js" crossorigin></script>
    <script src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js" crossorigin></script>
    <script src="https://unpkg.com/@remix-run/router@1.16.1/dist/router.umd.min.js" crossorigin></script>
    <script src="https://unpkg.com/react-router@6.23.1/dist/umd/react-router.production.min.js" crossorigin></script>
    <script src="https://unpkg.com/react-router-dom@6.23.1/dist/umd/react-router-dom.production.min.js" crossorigin></script>
</head>
<body>
    <div id="root"></div>
    <script type="module" src="/assets/index.js"></script>
</body>
</html>`

	// Extrac global elements
	globalCSS := ""
	if gs, ok := definition["global_styles"].(string); ok {
		globalCSS = gs
	}
	files["index.css"] = globalCSS

	// Generate index.jsx (Entry point)
	var indexBuilder strings.Builder
	indexBuilder.WriteString(`const { createRoot } = ReactDOM;
const root = createRoot(document.getElementById('root'));
import { App } from './App.jsx';
root.render(React.createElement(App, null));
`)
	files["index.jsx"] = indexBuilder.String()

	// Generate App.jsx (Router)
	var appBuilder strings.Builder
	appBuilder.WriteString(`const { BrowserRouter, Routes, Route } = ReactRouterDOM;
`)

	var pages []map[string]any
	if p, ok := definition["pages"].([]any); ok {
		for _, pi := range p {
			if pm, ok := pi.(map[string]any); ok {
				pages = append(pages, pm)
			}
		}
	}

	for i, page := range pages {
		route := getString(page, "route")
		if route == "" {
			route = "/"
		}
		
		componentTree := getList(page, "component_tree")
		pageName := fmt.Sprintf("Page%d", i)
		appBuilder.WriteString(fmt.Sprintf("import { %s } from './%s.jsx';\n", pageName, pageName))
		
		// Generate the specific page component file based on AST tree
		files[pageName+".jsx"] = generatePageComponent(pageName, componentTree)
	}

	appBuilder.WriteString(`
export function App() {
	return (
		<BrowserRouter>
			<Routes>
`)

	for i, page := range pages {
		route := getString(page, "route")
		if route == "" { route = "/" }
		pageName := fmt.Sprintf("Page%d", i)
		appBuilder.WriteString(fmt.Sprintf("				<Route path=\"%s\" element={<%s />} />\n", route, pageName))
	}

	appBuilder.WriteString(`			</Routes>
		</BrowserRouter>
	);
}
`)
	
	// Inject Dev Server HMR WebSocket bindings if development mode
	if isDevelopment {
		projectID := ""
		if id, ok := definition["id"].(string); ok {
			projectID = id
		}
		appBuilder.WriteString(fmt.Sprintf(`
// Development HMR Connector
if (window.location.hostname === '127.0.0.1' || window.location.hostname === 'localhost') {
    const ws = new WebSocket('ws://127.0.0.1:8080/api/v1/landing-pages/%s/dev-server/hmr');
    ws.onmessage = (event) => {
        if (event.data === 'reload') {
            window.location.reload();
        }
    };
    ws.onclose = () => console.log('HMR Disconnected');
}
`, projectID))
	}

	files["App.jsx"] = appBuilder.String()

	return files, nil
}

// generatePageComponent iterates the AST component tree directly into a hard-coded React functional component
func generatePageComponent(componentName string, tree []map[string]any) string {
	var out strings.Builder
	out.WriteString(fmt.Sprintf("export function %s() {\n", componentName))
	
	// Stub action handlers
	out.WriteString(`
    const handleSubmit = async (e, actionUrl) => {
        e.preventDefault();
        const formData = new FormData(e.target);
        const data = Object.fromEntries(formData.entries());
        
        try {
            const res = await fetch(actionUrl, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
            const result = await res.json();
            if(result.redirect) {
                window.location.href = result.redirect;
            }
        } catch(err) {
            console.error(err);
        }
    };
`)

	out.WriteString("\treturn (\n\t\t<React.Fragment>\n")
	for _, node := range tree {
		out.WriteString(renderNode(node, 3))
	}
	out.WriteString("\t\t</React.Fragment>\n\t);\n}\n")
	return out.String()
}

func renderNode(node map[string]any, indent int) string {
	tabs := strings.Repeat("\t", indent)
	nodeType := getString(node, "type")
	props := getMap(node, "properties")
	children := getList(node, "children")

	// Parse arbitrary raw styles back into JSON object for React style={} prop
	styleStr := getString(props, "inline_style")
	styleMap := getMapObj(props, "style")

	if styleStr != "" && len(styleMap) == 0 {
		parts := strings.Split(styleStr, ";")
		for _, part := range parts {
			if strings.Contains(part, ":") {
				kv := strings.SplitN(part, ":", 2)
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				words := strings.Split(k, "-")
				for i := 1; i < len(words); i++ {
					words[i] = strings.Title(words[i])
				}
				camelK := strings.Join(words, "")
				styleMap[camelK] = v
			}
		}
	}

	styleObj := "{}"
	if len(styleMap) > 0 {
		if b, err := json.Marshal(styleMap); err == nil {
			styleObj = string(b)
		}
	}

	nodeID := getString(node, "component_id")
	classPrefix := ""
	if nodeID != "" {
		classPrefix = fmt.Sprintf(" className=\"css-%s\"", nodeID)
	}

	hoverStyles := getMapObj(props, "hover_style")
	activeStyles := getMapObj(props, "active_style")
	var dynamicStyles string
	if len(hoverStyles) > 0 || len(activeStyles) > 0 {
		var sb strings.Builder
		if len(hoverStyles) > 0 {
			sb.WriteString(fmt.Sprintf(".css-%s:hover { ", nodeID))
			for k, v := range hoverStyles {
				sb.WriteString(fmt.Sprintf("%s: %s !important; ", toCSSKebab(k), v))
			}
			sb.WriteString("} ")
		}
		if len(activeStyles) > 0 {
			sb.WriteString(fmt.Sprintf(".css-%s:active { ", nodeID))
			for k, v := range activeStyles {
				sb.WriteString(fmt.Sprintf("%s: %s !important; ", toCSSKebab(k), v))
			}
			sb.WriteString("} ")
		}
		dynamicStyles = fmt.Sprintf("\n%s<style dangerouslySetInnerHTML={{ __html: `%s` }} />\n", tabs+"\t", sb.String())
	}

	content := getString(props, "content")
	if content == "" {
		content = getString(props, "text")
	}
	
	switch nodeType {
	case "container", "row", "column", "tabs", "accordion":
		out := fmt.Sprintf("%s<div%s style={%s}>\n%s", tabs, classPrefix, styleObj, dynamicStyles)
		for _, child := range children {
			out += renderNode(child, indent+1)
		}
		out += fmt.Sprintf("%s</div>\n", tabs)
		return out
	case "navbar":
		out := fmt.Sprintf("%s<nav%s style={%s}>\n%s", tabs, classPrefix, styleObj, dynamicStyles)
		for _, child := range children {
			out += renderNode(child, indent+1)
		}
		out += fmt.Sprintf("%s</nav>\n", tabs)
		return out
	case "footer":
		out := fmt.Sprintf("%s<footer%s style={%s}>\n%s", tabs, classPrefix, styleObj, dynamicStyles)
		for _, child := range children {
			out += renderNode(child, indent+1)
		}
		out += fmt.Sprintf("%s</footer>\n", tabs)
		return out
	case "text":
		return fmt.Sprintf("%s<div%s style={%s}>%s%s</div>\n", tabs, classPrefix, styleObj, dynamicStyles, content)
	case "heading":
		level := getString(props, "level")
		if level == "" { level = "h2" }
		return fmt.Sprintf("%s<%s%s style={%s}>%s%s</%s>\n", tabs, level, classPrefix, styleObj, dynamicStyles, content, level)
	case "paragraph":
		return fmt.Sprintf("%s<p%s style={%s}>%s%s</p>\n", tabs, classPrefix, styleObj, dynamicStyles, content)
	case "form":
		action := getString(props, "action")
		if action == "" { action = "/api/submit" }
		out := fmt.Sprintf("%s<form%s style={%s} onSubmit={(e) => handleSubmit(e, '%s')}>\n%s", tabs, classPrefix, styleObj, action, dynamicStyles)
		for _, child := range children {
			out += renderNode(child, indent+1)
		}
		out += fmt.Sprintf("%s</form>\n", tabs)
		return out
	case "input", "text_input", "email_input", "password_input":
		name := getString(props, "name")
		if name == "" { name = fmt.Sprintf("input_%d", len(content)) }
		placeholder := getString(props, "placeholder")
		inputType := getString(props, "type")
		if inputType == "" { 
			if nodeType == "email_input" { inputType = "email" } else if nodeType == "password_input" { inputType = "password" } else { inputType = "text" }
		}
		return fmt.Sprintf("%s<input%s type='%s' name='%s' placeholder='%s' style={%s} />\n%s", tabs, classPrefix, inputType, name, placeholder, styleObj, dynamicStyles)
	case "select":
		name := getString(props, "name")
		options := getList(props, "options")
		var optsStr strings.Builder
		for _, opt := range options {
			optsStr.WriteString(fmt.Sprintf("\n%s\t<option value=\"%s\">%s</option>", tabs, getString(opt, "value"), getString(opt, "label")))
		}
		return fmt.Sprintf("%s<select%s name=\"%s\" style={%s}>%s%s\n%s</select>\n", tabs, classPrefix, name, styleObj, dynamicStyles, optsStr.String(), tabs)
	case "checkbox", "radio":
		options := getList(props, "options")
		name := getString(props, "name")
		var optsStr strings.Builder
		for i, opt := range options {
			optsStr.WriteString(fmt.Sprintf("\n%s\t<label key=\"%d\" className=\"flex items-center gap-2\"><input type=\"%s\" name=\"%s\" value=\"%s\" /> %s</label>", tabs, i, nodeType, name, getString(opt, "value"), getString(opt, "label")))
		}
		return fmt.Sprintf("%s<fieldset%s style={%s}>%s%s\n%s</fieldset>\n", tabs, classPrefix, styleObj, dynamicStyles, optsStr.String(), tabs)
	case "video_embed":
		src := getString(props, "src")
		return fmt.Sprintf("%s<iframe%s src=\"%s\" style={%s}>%s</iframe>\n", tabs, classPrefix, src, styleObj, dynamicStyles)
	case "image", "logo":
		src := getString(props, "src")
		alt := getString(props, "alt")
		return fmt.Sprintf("%s<img%s src=\"%s\" alt=\"%s\" style={%s} />\n%s", tabs, classPrefix, src, alt, styleObj, dynamicStyles)
	case "divider":
		return fmt.Sprintf("%s<hr%s style={%s} />\n%s", tabs, classPrefix, styleObj, dynamicStyles)
	case "button", "submit_button":
		buttonType := "button"
		if nodeType == "submit_button" || getString(props, "action") == "submit" { buttonType = "submit" }
		return fmt.Sprintf("%s<button%s type='%s' style={%s}>%s%s</button>\n", tabs, classPrefix, buttonType, styleObj, dynamicStyles, content)
	default: // fallback to div
		out := fmt.Sprintf("%s<div%s style={%s}>\n%s", tabs, classPrefix, styleObj, dynamicStyles)
		if content != "" {
			out += fmt.Sprintf("%s\t%s\n", tabs, content)
		}
		for _, child := range children {
			out += renderNode(child, indent+1)
		}
		out += fmt.Sprintf("%s</div>\n", tabs)
		return out
	}
}

// Utility extractors
func getString(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok { return val }
	return ""
}
func getMap(m map[string]any, key string) map[string]any {
	if val, ok := m[key].(map[string]any); ok { return val }
	return make(map[string]any)
}
func getMapObj(m map[string]any, key string) map[string]string {
	res := make(map[string]string)
	if val, ok := m[key].(map[string]any); ok {
		for k, v := range val {
			res[k] = fmt.Sprintf("%v", v)
		}
	}
	return res
}
func getList(m map[string]any, key string) []map[string]any {
	var res []map[string]any
	if val, ok := m[key].([]any); ok {
		for _, item := range val {
			if im, ok := item.(map[string]any); ok {
				res = append(res, im)
			}
		}
	}
	return res
}
func toCSSKebab(camel string) string {
	var sb strings.Builder
	for i, c := range camel {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				sb.WriteRune('-')
			}
			sb.WriteRune(c + 32)
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

