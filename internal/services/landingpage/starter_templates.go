package landingpage

import "github.com/google/uuid"

// GetStarterTemplates returns the 5 pre-built multi-page flow templates.
func GetStarterTemplates() []StarterTemplate {
	return []StarterTemplate{
		loginLoadingSuccess(),
		loginMFADashboard(),
		ssoConsentRedirect(),
		fileShareLoginDownload(),
		passwordResetConfirmation(),
	}
}

// StarterTemplate represents a pre-built template definition.
type StarterTemplate struct {
	Name        string
	Description string
	Category    string
	Definition  map[string]any
}

func loginLoadingSuccess() StarterTemplate {
	return StarterTemplate{
		Name:        "Login → Loading → Success",
		Description: "Standard login page with loading animation and success confirmation.",
		Category:    "starter",
		Definition: map[string]any{
			"schema_version": 1,
			"pages": []any{
				makeLoginPage("page-1", "/", "Sign In"),
				makeLoadingPage("page-2", "/loading"),
				makeSuccessPage("page-3", "/success"),
			},
			"global_styles": defaultGlobalStyles(),
			"global_js":     "",
			"theme":         map[string]any{},
			"navigation": []any{
				map[string]any{"source_page": "page-1", "trigger": "form_submit", "target_page": "page-2", "delay_ms": 0},
				map[string]any{"source_page": "page-2", "trigger": "redirect", "target_page": "page-3", "delay_ms": 3000},
			},
		},
	}
}

func loginMFADashboard() StarterTemplate {
	return StarterTemplate{
		Name:        "Login → MFA → Dashboard",
		Description: "Login with multi-factor authentication step before dashboard.",
		Category:    "starter",
		Definition: map[string]any{
			"schema_version": 1,
			"pages": []any{
				makeLoginPage("page-1", "/", "Sign In"),
				makeMFAPage("page-2", "/verify"),
				makeDashboardPage("page-3", "/dashboard"),
			},
			"global_styles": defaultGlobalStyles(),
			"global_js":     "",
			"theme":         map[string]any{},
			"navigation": []any{
				map[string]any{"source_page": "page-1", "trigger": "form_submit", "target_page": "page-2", "delay_ms": 0},
				map[string]any{"source_page": "page-2", "trigger": "form_submit", "target_page": "page-3", "delay_ms": 0},
			},
		},
	}
}

func ssoConsentRedirect() StarterTemplate {
	return StarterTemplate{
		Name:        "SSO Login → Consent → Redirect",
		Description: "Single sign-on login with OAuth consent screen and redirect.",
		Category:    "starter",
		Definition: map[string]any{
			"schema_version": 1,
			"pages": []any{
				makeLoginPage("page-1", "/", "Sign In with SSO"),
				makeConsentPage("page-2", "/consent"),
				makeRedirectPage("page-3", "/redirect"),
			},
			"global_styles": defaultGlobalStyles(),
			"global_js":     "",
			"theme":         map[string]any{},
			"navigation": []any{
				map[string]any{"source_page": "page-1", "trigger": "form_submit", "target_page": "page-2", "delay_ms": 0},
				map[string]any{"source_page": "page-2", "trigger": "click", "target_page": "page-3", "delay_ms": 0},
			},
		},
	}
}

func fileShareLoginDownload() StarterTemplate {
	return StarterTemplate{
		Name:        "File Share Login → Download Page",
		Description: "File sharing service login page with download confirmation.",
		Category:    "starter",
		Definition: map[string]any{
			"schema_version": 1,
			"pages": []any{
				makeLoginPage("page-1", "/", "Access Shared File"),
				makeDownloadPage("page-2", "/download"),
			},
			"global_styles": defaultGlobalStyles(),
			"global_js":     "",
			"theme":         map[string]any{},
			"navigation": []any{
				map[string]any{"source_page": "page-1", "trigger": "form_submit", "target_page": "page-2", "delay_ms": 0},
			},
		},
	}
}

func passwordResetConfirmation() StarterTemplate {
	return StarterTemplate{
		Name:        "Password Reset → Confirmation",
		Description: "Password reset form with confirmation page.",
		Category:    "starter",
		Definition: map[string]any{
			"schema_version": 1,
			"pages": []any{
				makePasswordResetPage("page-1", "/", "Reset Your Password"),
				makeConfirmationPage("page-2", "/confirmed"),
			},
			"global_styles": defaultGlobalStyles(),
			"global_js":     "",
			"theme":         map[string]any{},
			"navigation": []any{
				map[string]any{"source_page": "page-1", "trigger": "form_submit", "target_page": "page-2", "delay_ms": 0},
			},
		},
	}
}

// ---------- Page Builders ----------

func makeLoginPage(pageID, route, title string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Login",
		"route":     route,
		"title":     title,
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "login-container",
					"inline_style": "max-width:400px;margin:80px auto;padding:40px;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": title, "level": "h1"}),
					makeComponent("email_input", map[string]any{"name": "email", "placeholder": "Email address", "capture_tag": "email"}),
					makeComponent("password_input", map[string]any{"name": "password", "placeholder": "Password", "capture_tag": "password"}),
					makeComponent("submit_button", map[string]any{"content": "Sign In"}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeMFAPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "MFA Verification",
		"route":     route,
		"title":     "Verify Your Identity",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "mfa-container",
					"inline_style": "max-width:400px;margin:80px auto;padding:40px;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Verify Your Identity", "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "Enter the verification code sent to your device."}),
					makeComponent("text_input", map[string]any{"name": "mfa_code", "placeholder": "Enter code", "capture_tag": "mfa_token"}),
					makeComponent("submit_button", map[string]any{"content": "Verify"}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeLoadingPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Loading",
		"route":     route,
		"title":     "Signing in...",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "loading-container",
					"inline_style": "text-align:center;margin:120px auto;",
				},
				"children": []any{
					makeComponent("spinner", map[string]any{}),
					makeComponent("paragraph", map[string]any{"content": "Signing you in..."}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeSuccessPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Success",
		"route":     route,
		"title":     "Success",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "success-container",
					"inline_style": "text-align:center;margin:120px auto;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Sign In Successful", "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "You are now signed in. Redirecting..."}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeDashboardPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Dashboard",
		"route":     route,
		"title":     "Dashboard",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties":   map[string]any{"css_class": "dashboard-container"},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Welcome to Your Dashboard", "level": "h1"}),
					makeComponent("paragraph", map[string]any{"content": "You have been successfully authenticated."}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeConsentPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Consent",
		"route":     route,
		"title":     "Grant Access",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "consent-container",
					"inline_style": "max-width:500px;margin:80px auto;padding:40px;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Grant Access", "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "An application is requesting access to your account. Review the permissions below."}),
					makeComponent("checkbox", map[string]any{"name": "consent_read", "label": "Read your profile information"}),
					makeComponent("checkbox", map[string]any{"name": "consent_email", "label": "Access your email address"}),
					makeComponent("button", map[string]any{"content": "Allow Access"}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeRedirectPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Redirect",
		"route":     route,
		"title":     "Redirecting...",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"inline_style": "text-align:center;margin:120px auto;",
				},
				"children": []any{
					makeComponent("spinner", map[string]any{}),
					makeComponent("paragraph", map[string]any{"content": "Redirecting you back to the application..."}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeDownloadPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Download",
		"route":     route,
		"title":     "Download File",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "download-container",
					"inline_style": "max-width:500px;margin:80px auto;padding:40px;text-align:center;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Your File is Ready", "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "Click below to download the shared file."}),
					makeComponent("button", map[string]any{"content": "Download File"}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makePasswordResetPage(pageID, route, title string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Password Reset",
		"route":     route,
		"title":     title,
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"css_class":    "reset-container",
					"inline_style": "max-width:400px;margin:80px auto;padding:40px;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": title, "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "Enter your new password below."}),
					makeComponent("password_input", map[string]any{"name": "new_password", "placeholder": "New password", "capture_tag": "password"}),
					makeComponent("password_input", map[string]any{"name": "confirm_password", "placeholder": "Confirm password", "capture_tag": "password"}),
					makeComponent("submit_button", map[string]any{"content": "Reset Password"}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeConfirmationPage(pageID, route string) map[string]any {
	return map[string]any{
		"page_id":   pageID,
		"name":      "Confirmation",
		"route":     route,
		"title":     "Password Updated",
		"favicon":   "",
		"meta_tags": []any{},
		"component_tree": []any{
			map[string]any{
				"component_id": uuid.New().String(),
				"type":         "container",
				"properties": map[string]any{
					"inline_style": "text-align:center;margin:120px auto;",
				},
				"children": []any{
					makeComponent("heading", map[string]any{"content": "Password Updated", "level": "h2"}),
					makeComponent("paragraph", map[string]any{"content": "Your password has been successfully changed. You can now sign in with your new password."}),
				},
				"event_bindings": []any{},
			},
		},
		"page_styles": "",
		"page_js":     "",
	}
}

func makeComponent(cType string, props map[string]any) map[string]any {
	return map[string]any{
		"component_id":   uuid.New().String(),
		"type":           cType,
		"properties":     props,
		"children":       []any{},
		"event_bindings": []any{},
	}
}

func defaultGlobalStyles() string {
	return `* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
input, select, textarea { width: 100%; padding: 10px 12px; margin: 8px 0; border: 1px solid #ddd; border-radius: 4px; font-size: 14px; }
button[type="submit"], .btn-primary { width: 100%; padding: 12px; margin-top: 16px; background: #0066cc; color: #fff; border: none; border-radius: 4px; font-size: 16px; cursor: pointer; }
button[type="submit"]:hover, .btn-primary:hover { background: #0052a3; }
h1, h2, h3 { margin-bottom: 16px; }
p { margin-bottom: 12px; }`
}
