package landingpage

import (
	"fmt"
	"html"
	"strings"
)

// RenderPreviewHTML generates an HTML preview from a page definition.
// pageIndex selects which page to render (0-based); defaults to 0.
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
	pageStyles, _ := page["page_styles"].(string)
	pageJS, _ := page["page_js"].(string)
	globalStyles, _ := def["global_styles"].(string)
	globalJS, _ := def["global_js"].(string)

	var body strings.Builder
	if tree, ok := page["component_tree"].([]any); ok {
		for _, comp := range tree {
			renderComponent(&body, comp, 0)
		}
	}

	// Render meta tags.
	var metaTags strings.Builder
	if metas, ok := page["meta_tags"].([]any); ok {
		for _, m := range metas {
			if meta, ok := m.(map[string]any); ok {
				name, _ := meta["name"].(string)
				content, _ := meta["content"].(string)
				if name != "" {
					metaTags.WriteString(fmt.Sprintf(`    <meta name="%s" content="%s">`, html.EscapeString(name), html.EscapeString(content)))
					metaTags.WriteString("\n")
				}
			}
		}
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
%s%s
    <style>%s</style>
    <style>%s</style>
</head>
<body>
    <div style="position:fixed;top:0;left:0;right:0;background:#ff6b00;color:#fff;text-align:center;padding:4px 0;font-family:sans-serif;font-size:12px;font-weight:bold;z-index:99999;">PREVIEW MODE</div>
    <div style="padding-top:24px;">
%s
    </div>
    <script>%s</script>
    <script>%s</script>
</body>
</html>`,
		html.EscapeString(title),
		faviconTag,
		metaTags.String(),
		globalStyles,
		pageStyles,
		body.String(),
		globalJS,
		pageJS,
	), nil
}

func renderComponent(w *strings.Builder, comp any, depth int) {
	c, ok := comp.(map[string]any)
	if !ok {
		return
	}

	cType, _ := c["type"].(string)
	props, _ := c["properties"].(map[string]any)
	children, _ := c["children"].([]any)

	// Skip hidden components in preview output.
	if hidden, ok := props["hidden"].(bool); ok && hidden {
		return
	}

	indent := strings.Repeat("        ", depth+1)

	// Get common properties.
	cssClass, _ := props["css_class"].(string)
	inlineStyle, _ := props["inline_style"].(string)
	id, _ := props["id"].(string)

	attrs := buildAttrs(id, cssClass, inlineStyle)

	switch cType {
	case "container", "section", "card":
		tag := "div"
		if cType == "section" {
			tag = "section"
		}
		w.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, attrs))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	case "row":
		rowStyle := mergePreviewStyles("display:flex;", inlineStyle)
		w.WriteString(fmt.Sprintf("%s<div%s>\n", indent, buildAttrs(id, cssClass, rowStyle)))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</div>\n", indent))

	case "column":
		colStyle := mergePreviewStyles("flex:1;", inlineStyle)
		w.WriteString(fmt.Sprintf("%s<div%s>\n", indent, buildAttrs(id, cssClass, colStyle)))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</div>\n", indent))

	case "heading":
		level := "h2"
		if l, ok := props["level"].(string); ok && l != "" {
			level = l
		} else if l, ok := props["level"].(float64); ok {
			level = fmt.Sprintf("h%d", int(l))
		}
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<%s%s>%s</%s>\n", indent, level, attrs, html.EscapeString(content), level))

	case "paragraph":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<p%s>%s</p>\n", indent, attrs, html.EscapeString(content)))

	case "span":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<span%s>%s</span>\n", indent, attrs, html.EscapeString(content)))

	case "label":
		content, _ := props["content"].(string)
		forAttr, _ := props["for"].(string)
		extra := ""
		if forAttr != "" {
			extra = fmt.Sprintf(` for="%s"`, html.EscapeString(forAttr))
		}
		w.WriteString(fmt.Sprintf("%s<label%s%s>%s</label>\n", indent, attrs, extra, html.EscapeString(content)))

	case "blockquote":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<blockquote%s>%s</blockquote>\n", indent, attrs, html.EscapeString(content)))

	case "code_block":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<pre%s><code>%s</code></pre>\n", indent, attrs, html.EscapeString(content)))

	case "image":
		src, _ := props["src"].(string)
		alt, _ := props["alt"].(string)
		w.WriteString(fmt.Sprintf(`%s<img%s src="%s" alt="%s">`+"\n", indent, attrs, html.EscapeString(src), html.EscapeString(alt)))

	case "video":
		src, _ := props["src"].(string)
		w.WriteString(fmt.Sprintf(`%s<video%s src="%s" controls></video>`+"\n", indent, attrs, html.EscapeString(src)))

	case "icon", "logo":
		src, _ := props["src"].(string)
		alt, _ := props["alt"].(string)
		if alt == "" {
			alt = cType
		}
		w.WriteString(fmt.Sprintf(`%s<img%s src="%s" alt="%s">`+"\n", indent, attrs, html.EscapeString(src), html.EscapeString(alt)))

	case "iframe":
		src, _ := props["src"].(string)
		width, _ := props["width"].(string)
		height, _ := props["height"].(string)
		sandbox, _ := props["sandbox"].(string)
		allow, _ := props["allow"].(string)
		isOverlay, _ := props["is_overlay"].(bool)
		if width == "" {
			width = "100%"
		}
		if height == "" {
			height = "400"
		}
		if isOverlay {
			overlayPos := "absolute"
			if pos, ok := props["overlay_position"].(string); ok && pos != "" {
				overlayPos = pos
			}
			overlayOpacity := 0.01
			if op, ok := props["overlay_opacity"].(float64); ok {
				overlayOpacity = op
			}
			overlayZ := 9999
			if z, ok := props["overlay_z_index"].(float64); ok {
				overlayZ = int(z)
			}
			overlayPE := "auto"
			if pe, ok := props["overlay_pointer_events"].(string); ok && pe != "" {
				overlayPE = pe
			}
			overlayCss := fmt.Sprintf("position:%s;top:0;left:0;width:100%%;height:100%%;opacity:%.2f;z-index:%d;pointer-events:%s;border:none",
				overlayPos, overlayOpacity, overlayZ, overlayPE)
			mergedStyle := inlineStyle
			if mergedStyle != "" {
				mergedStyle = mergedStyle + ";" + overlayCss
			} else {
				mergedStyle = overlayCss
			}
			overlayAttrs := buildAttrs(id, cssClass, mergedStyle)
			iframeAttrs := fmt.Sprintf(` src="%s"`, html.EscapeString(src))
			if sandbox != "" {
				iframeAttrs += fmt.Sprintf(` sandbox="%s"`, html.EscapeString(sandbox))
			}
			if allow != "" {
				iframeAttrs += fmt.Sprintf(` allow="%s"`, html.EscapeString(allow))
			}
			w.WriteString(fmt.Sprintf("%s<iframe%s%s></iframe>\n", indent, overlayAttrs, iframeAttrs))
		} else {
			iframeAttrs := fmt.Sprintf(` src="%s" width="%s" height="%s"`,
				html.EscapeString(src), html.EscapeString(width), html.EscapeString(height))
			if sandbox != "" {
				iframeAttrs += fmt.Sprintf(` sandbox="%s"`, html.EscapeString(sandbox))
			}
			if allow != "" {
				iframeAttrs += fmt.Sprintf(` allow="%s"`, html.EscapeString(allow))
			}
			w.WriteString(fmt.Sprintf("%s<iframe%s%s></iframe>\n", indent, attrs, iframeAttrs))
		}

	case "text_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		w.WriteString(fmt.Sprintf(`%s<input type="text"%s name="%s" placeholder="%s">`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder)))

	case "password_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		w.WriteString(fmt.Sprintf(`%s<input type="password"%s name="%s" placeholder="%s">`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder)))

	case "email_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		w.WriteString(fmt.Sprintf(`%s<input type="email"%s name="%s" placeholder="%s">`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder)))

	case "textarea":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		w.WriteString(fmt.Sprintf(`%s<textarea%s name="%s" placeholder="%s"></textarea>`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder)))

	case "select":
		name, _ := props["name"].(string)
		w.WriteString(fmt.Sprintf(`%s<select%s name="%s">`, indent, attrs, html.EscapeString(name)))
		if options, ok := props["options"].([]any); ok {
			for _, opt := range options {
				if o, ok := opt.(map[string]any); ok {
					val, _ := o["value"].(string)
					label, _ := o["label"].(string)
					w.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, html.EscapeString(val), html.EscapeString(label)))
				}
			}
		}
		w.WriteString("</select>\n")

	case "checkbox":
		name, _ := props["name"].(string)
		labelText, _ := props["label"].(string)
		w.WriteString(fmt.Sprintf(`%s<label%s><input type="checkbox" name="%s"> %s</label>`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(labelText)))

	case "radio":
		name, _ := props["name"].(string)
		value, _ := props["value"].(string)
		labelText, _ := props["label"].(string)
		w.WriteString(fmt.Sprintf(`%s<label%s><input type="radio" name="%s" value="%s"> %s</label>`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(value), html.EscapeString(labelText)))

	case "file_upload":
		name, _ := props["name"].(string)
		w.WriteString(fmt.Sprintf(`%s<input type="file"%s name="%s">`+"\n", indent, attrs, html.EscapeString(name)))

	case "hidden_field":
		name, _ := props["name"].(string)
		value, _ := props["value"].(string)
		w.WriteString(fmt.Sprintf(`%s<input type="hidden"%s name="%s" value="%s">`+"\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(value)))

	case "button":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf(`%s<button type="button"%s>%s</button>`+"\n", indent, attrs, html.EscapeString(content)))

	case "submit_button":
		content, _ := props["content"].(string)
		if content == "" {
			content = "Submit"
		}
		w.WriteString(fmt.Sprintf(`%s<button type="submit"%s>%s</button>`+"\n", indent, attrs, html.EscapeString(content)))

	case "link":
		href, _ := props["href"].(string)
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf(`%s<a%s href="%s">%s</a>`+"\n", indent, attrs, html.EscapeString(href), html.EscapeString(content)))

	case "toggle":
		name, _ := props["name"].(string)
		w.WriteString(fmt.Sprintf(`%s<label%s><input type="checkbox" name="%s" role="switch"> Toggle</label>`+"\n",
			indent, attrs, html.EscapeString(name)))

	case "alert":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf(`%s<div%s role="alert">%s</div>`+"\n", indent, attrs, html.EscapeString(content)))

	case "spinner":
		w.WriteString(fmt.Sprintf(`%s<div%s class="spinner">Loading...</div>`+"\n", indent, attrs))

	case "progress_bar":
		value, _ := props["value"].(float64)
		w.WriteString(fmt.Sprintf(`%s<progress%s value="%d" max="100"></progress>`+"\n", indent, attrs, int(value)))

	case "toast":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf(`%s<div%s role="status">%s</div>`+"\n", indent, attrs, html.EscapeString(content)))

	case "spacer":
		height := "20px"
		if h, ok := props["height"].(string); ok && h != "" {
			height = h
		}
		w.WriteString(fmt.Sprintf(`%s<div%s style="height:%s"></div>`+"\n", indent, attrs, html.EscapeString(height)))

	case "divider":
		w.WriteString(fmt.Sprintf("%s<hr%s>\n", indent, attrs))

	case "navbar":
		w.WriteString(fmt.Sprintf("%s<nav%s>\n", indent, attrs))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</nav>\n", indent))

	case "footer":
		w.WriteString(fmt.Sprintf("%s<footer%s>\n", indent, attrs))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</footer>\n", indent))

	case "breadcrumb":
		w.WriteString(fmt.Sprintf(`%s<nav%s aria-label="breadcrumb"><ol></ol></nav>`+"\n", indent, attrs))

	case "tabs":
		w.WriteString(fmt.Sprintf(`%s<div%s role="tablist">`+"\n", indent, attrs))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</div>\n", indent))

	case "sidebar":
		w.WriteString(fmt.Sprintf("%s<aside%s>\n", indent, attrs))
		renderChildren(w, children, depth+1)
		w.WriteString(fmt.Sprintf("%s</aside>\n", indent))

	case "raw_html":
		content, _ := props["content"].(string)
		w.WriteString(indent + content + "\n")

	default:
		w.WriteString(fmt.Sprintf("%s<!-- unknown component: %s -->\n", indent, html.EscapeString(cType)))
	}
}

// mergePreviewStyles prepends default CSS properties before user inline styles.
func mergePreviewStyles(defaults, userStyle string) string {
	if userStyle == "" {
		return defaults
	}
	d := strings.TrimRight(defaults, " ;")
	if d != "" {
		d += ";"
	}
	return d + userStyle
}

func renderChildren(w *strings.Builder, children []any, depth int) {
	for _, child := range children {
		renderComponent(w, child, depth)
	}
}

func buildAttrs(id, cssClass, inlineStyle string) string {
	var parts []string
	if id != "" {
		parts = append(parts, fmt.Sprintf(`id="%s"`, html.EscapeString(id)))
	}
	if cssClass != "" {
		parts = append(parts, fmt.Sprintf(`class="%s"`, html.EscapeString(cssClass)))
	}
	if inlineStyle != "" {
		parts = append(parts, fmt.Sprintf(`style="%s"`, html.EscapeString(inlineStyle)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}
