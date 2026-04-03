package landingpage

import (
	"fmt"
	"strings"
)

// MaxNestingDepth is the maximum allowed nesting depth for components.
const MaxNestingDepth = 20

// MaxPagesPerProject is the maximum number of pages in a project.
const MaxPagesPerProject = 50

// ValidComponentTypes is the set of all recognized component types.
var ValidComponentTypes = map[string]bool{
	// Layout
	"container": true, "row": true, "column": true, "section": true,
	"spacer": true, "divider": true, "card": true,
	// Navigation
	"navbar": true, "footer": true, "breadcrumb": true, "tabs": true, "sidebar": true,
	// Text
	"heading": true, "paragraph": true, "text": true, "span": true, "label": true,
	"blockquote": true, "code_block": true,
	// Media
	"image": true, "icon": true, "video": true, "logo": true, "iframe": true,
	// Form Elements
	"form": true,
	"text_input": true, "password_input": true, "email_input": true,
	"textarea": true, "select": true, "checkbox": true, "radio": true,
	"file_upload": true, "hidden_field": true,
	// Interactive
	"button": true, "link": true, "submit_button": true, "toggle": true,
	// Feedback
	"alert": true, "spinner": true, "progress_bar": true, "toast": true,
	// Special
	"raw_html": true,
}

// ValidCaptureTags is the set of recognized capture tag values for form fields.
var ValidCaptureTags = map[string]bool{
	"username": true, "password": true, "email": true, "mfa_token": true, "custom": true,
}

// ValidEventTypes is the set of recognized event binding types.
var ValidEventTypes = map[string]bool{
	// React-style
	"onClick": true, "onSubmit": true, "onFocus": true, "onBlur": true,
	"onInput": true, "onChange": true, "onLoad": true, "onMouseEnter": true,
	"onMouseLeave": true,
	// DOM-style (used in builder definitions and seed data)
	"click": true, "submit": true, "focus": true, "blur": true,
	"input": true, "change": true, "load": true, "mouseenter": true,
	"mouseleave": true,
}

// NestableComponentTypes are component types that can contain children.
var NestableComponentTypes = map[string]bool{
	"container": true, "row": true, "column": true, "section": true,
	"card": true, "navbar": true, "footer": true, "sidebar": true,
	"tabs": true, "form": true,
}

// ValidateDefinition validates a page definition JSON structure.
func ValidateDefinition(def map[string]any) error {
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}

	// Check schema version.
	if v, ok := def["schema_version"]; ok {
		switch sv := v.(type) {
		case float64:
			if sv < 1 {
				return fmt.Errorf("schema_version must be >= 1")
			}
		case int:
			if sv < 1 {
				return fmt.Errorf("schema_version must be >= 1")
			}
		}
	}

	// Validate pages array.
	pages, ok := def["pages"]
	if !ok {
		return fmt.Errorf("pages array is required")
	}
	pagesArr, ok := pages.([]any)
	if !ok {
		return fmt.Errorf("pages must be an array")
	}
	if len(pagesArr) == 0 {
		return fmt.Errorf("at least one page is required")
	}
	if len(pagesArr) > MaxPagesPerProject {
		return fmt.Errorf("maximum %d pages per project", MaxPagesPerProject)
	}

	routeSeen := map[string]bool{}
	for i, pg := range pagesArr {
		page, ok := pg.(map[string]any)
		if !ok {
			return fmt.Errorf("page %d: must be an object", i)
		}

		if err := validatePage(page, i, routeSeen); err != nil {
			return err
		}
	}

	// Validate navigation flows if present.
	if nav, ok := def["navigation"]; ok {
		if navArr, ok := nav.([]any); ok {
			for i, n := range navArr {
				if err := validateNavigation(n, i); err != nil {
					return err
				}
			}
		}
	}

	// Validate tracking_config if present (optional — all fields default to true).
	if tc, ok := def["tracking_config"]; ok {
		if _, ok := tc.(map[string]any); !ok {
			return fmt.Errorf("tracking_config must be an object")
		}
	}

	return nil
}

func validatePage(page map[string]any, idx int, routeSeen map[string]bool) error {
	// page_id is required.
	pageID, _ := page["page_id"].(string)
	if pageID == "" {
		return fmt.Errorf("page %d: page_id is required", idx)
	}

	// name is required.
	name, _ := page["name"].(string)
	if name == "" {
		return fmt.Errorf("page %d: name is required", idx)
	}

	// route is required and must be unique.
	route, _ := page["route"].(string)
	if route == "" {
		return fmt.Errorf("page %d: route is required", idx)
	}
	if routeSeen[route] {
		return fmt.Errorf("page %d: duplicate route %q", idx, route)
	}
	routeSeen[route] = true

	// Validate component tree if present.
	if ct, ok := page["component_tree"]; ok {
		if tree, ok := ct.([]any); ok {
			for i, c := range tree {
				if err := validateComponent(c, fmt.Sprintf("page %d, component %d", idx, i), 1); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateComponent(comp any, path string, depth int) error {
	if depth > MaxNestingDepth {
		return fmt.Errorf("%s: nesting depth exceeds maximum of %d", path, MaxNestingDepth)
	}

	c, ok := comp.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: must be an object", path)
	}

	// component_id is required.
	if cid, _ := c["component_id"].(string); cid == "" {
		return fmt.Errorf("%s: component_id is required", path)
	}

	// type is required and must be valid.
	cType, _ := c["type"].(string)
	if cType == "" {
		return fmt.Errorf("%s: type is required", path)
	}
	if !ValidComponentTypes[cType] {
		return fmt.Errorf("%s: unknown component type %q", path, cType)
	}

	// Validate capture_tag on form elements.
	if props, ok := c["properties"].(map[string]any); ok {
		if tag, ok := props["capture_tag"].(string); ok && tag != "" {
			if !ValidCaptureTags[tag] {
				return fmt.Errorf("%s: unknown capture_tag %q", path, tag)
			}
		}
	}

	// Validate event bindings.
	if events, ok := c["event_bindings"].([]any); ok {
		for _, e := range events {
			if ev, ok := e.(map[string]any); ok {
				if evType, ok := ev["event"].(string); ok {
					if !ValidEventTypes[evType] {
						return fmt.Errorf("%s: unknown event type %q", path, evType)
					}
				}
			}
		}
	}

	// Validate children.
	if children, ok := c["children"].([]any); ok {
		if len(children) > 0 && !NestableComponentTypes[cType] {
			return fmt.Errorf("%s: component type %q cannot have children", path, cType)
		}
		for i, child := range children {
			childPath := fmt.Sprintf("%s > child %d", path, i)
			if err := validateComponent(child, childPath, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateNavigation(nav any, idx int) error {
	n, ok := nav.(map[string]any)
	if !ok {
		return fmt.Errorf("navigation %d: must be an object", idx)
	}

	trigger, _ := n["trigger"].(string)
	validTriggers := map[string]bool{
		"redirect": true, "click": true, "form_submit": true, "conditional": true,
	}
	if trigger != "" && !validTriggers[trigger] {
		return fmt.Errorf("navigation %d: unknown trigger %q", idx, trigger)
	}

	// component_id is optional — used by click and form_submit triggers for specificity.
	// If present, it must be a string.
	if cid, ok := n["component_id"]; ok {
		if _, isStr := cid.(string); !isStr {
			return fmt.Errorf("navigation %d: component_id must be a string", idx)
		}
	}

	return nil
}

// GetComponentTypes returns all available component types for the UI.
func GetComponentTypes() []ComponentTypeDTO {
	return []ComponentTypeDTO{
		// Layout
		{Type: "container", Category: "layout", Label: "Container", CanNest: true},
		{Type: "row", Category: "layout", Label: "Row", CanNest: true},
		{Type: "column", Category: "layout", Label: "Column", CanNest: true},
		{Type: "section", Category: "layout", Label: "Section", CanNest: true},
		{Type: "spacer", Category: "layout", Label: "Spacer"},
		{Type: "divider", Category: "layout", Label: "Divider"},
		{Type: "card", Category: "layout", Label: "Card", CanNest: true},
		// Navigation
		{Type: "navbar", Category: "navigation", Label: "Navigation Bar", CanNest: true},
		{Type: "footer", Category: "navigation", Label: "Footer", CanNest: true},
		{Type: "breadcrumb", Category: "navigation", Label: "Breadcrumb"},
		{Type: "tabs", Category: "navigation", Label: "Tabs", CanNest: true},
		{Type: "sidebar", Category: "navigation", Label: "Sidebar", CanNest: true},
		// Text
		{Type: "heading", Category: "text", Label: "Heading (H1-H6)"},
		{Type: "paragraph", Category: "text", Label: "Paragraph"},
		{Type: "text", Category: "text", Label: "Text"},
		{Type: "span", Category: "text", Label: "Span"},
		{Type: "label", Category: "text", Label: "Label"},
		{Type: "blockquote", Category: "text", Label: "Blockquote"},
		{Type: "code_block", Category: "text", Label: "Code Block"},
		// Media
		{Type: "image", Category: "media", Label: "Image"},
		{Type: "icon", Category: "media", Label: "Icon"},
		{Type: "video", Category: "media", Label: "Video Embed"},
		{Type: "logo", Category: "media", Label: "Logo"},
		{Type: "iframe", Category: "media", Label: "Iframe"},
		// Form Elements
		{Type: "form", Category: "form", Label: "Form", CanNest: true, HasCapture: true},
		{Type: "text_input", Category: "form", Label: "Text Input", HasCapture: true},
		{Type: "password_input", Category: "form", Label: "Password Input", HasCapture: true},
		{Type: "email_input", Category: "form", Label: "Email Input", HasCapture: true},
		{Type: "textarea", Category: "form", Label: "Textarea", HasCapture: true},
		{Type: "select", Category: "form", Label: "Select / Dropdown", HasCapture: true},
		{Type: "checkbox", Category: "form", Label: "Checkbox"},
		{Type: "radio", Category: "form", Label: "Radio Button"},
		{Type: "file_upload", Category: "form", Label: "File Upload"},
		{Type: "hidden_field", Category: "form", Label: "Hidden Field", HasCapture: true},
		// Interactive
		{Type: "button", Category: "interactive", Label: "Button"},
		{Type: "link", Category: "interactive", Label: "Link"},
		{Type: "submit_button", Category: "interactive", Label: "Submit Button"},
		{Type: "toggle", Category: "interactive", Label: "Toggle / Switch"},
		// Feedback
		{Type: "alert", Category: "feedback", Label: "Alert / Banner"},
		{Type: "spinner", Category: "feedback", Label: "Loading Spinner"},
		{Type: "progress_bar", Category: "feedback", Label: "Progress Bar"},
		{Type: "toast", Category: "feedback", Label: "Toast / Notification"},
		// Special
		{Type: "raw_html", Category: "special", Label: "Raw HTML Block"},
	}
}

// Suppress unused warning for strings import.
var _ = strings.TrimSpace
