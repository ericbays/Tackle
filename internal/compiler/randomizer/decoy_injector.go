// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// LiveDecoyInjector injects non-functional decoy content to vary output between builds.
type LiveDecoyInjector struct{}

// NewLiveDecoyInjector creates a new decoy injector.
func NewLiveDecoyInjector() *LiveDecoyInjector {
	return &LiveDecoyInjector{}
}

// InjectDecoys adds decoy HTML comments, hidden elements, unused CSS, and JS no-ops.
// Returns the modified HTML, CSS, JS, and a manifest of injections.
//
// The injection strategy includes:
//   - HTML comments (5-20 per build) with TODO notes, version strings, developer names,
//     lint markers, and section labels
//   - Hidden DOM elements (3-10 per build) using 5 hiding techniques: display:none,
//     visibility:hidden, off-screen, zero-size, and transparent
//   - Unused CSS rules (10-30 per build) targeting non-existent selectors
//   - JavaScript decoys (3-10 per build) including unused variables, empty functions,
//     dead code behind false conditions, no-op expressions, and try/catch blocks
//   - Meta tag variation (3-8 per build) with generator, viewport, robots, theme-color,
//     X-UA-Compatible, and format-detection tags
func (di *LiveDecoyInjector) InjectDecoys(html string, css string, js string, seed int64) (string, string, string, map[string]any, error) {
	// Create seeded random source for deterministic results
	src := rand.NewSource(seed)
	rd := rand.New(src)

	// Manifest to track all injection decisions
	manifest := map[string]any{}

	// Word pools for random comment generation (20+ words per pool)
	// TODO-related words
	todoPrefixes := []string{
		"refactor", "optimize", "add", "review", "update", "remove", "move", "split",
		"combine", "document", "test", "fix", "enable", "disable", "migrate", "upgrade",
		"downgrade", "restructure", "modernize", "simplify", "clean", "audit",
	}
	todoTopics := []string{
		"responsive breakpoints", "form validation", "theme colors", "media queries",
		"accessibility", "performance metrics", "animation frames", "focus states",
		"cross-browser compatibility", "SEO elements", "analytics tracking", "cookie consent",
		"dark mode support", "print styles", "font loading", "image optimization",
		"viewport units", "CSS custom properties", "grid layout", "flexbox fallbacks",
	}

	// Developer/author names
	developerNames := []string{
		"jsmith", "ajohnson", "bwilson", "clee", "dmartin", "ejones", "f Garcia",
		"ganderson", "htaylor", "ithomas", "jwhite", "kbrown", "lharris", "mclark",
		"ndixon", "ogreen", "pwalker", "qhall", "rallen", "ss young", "tbaker",
	}

	// Lint marker options
	lintMarkers := []string{
		"eslint-disable", "eslint-enable", "prettier-ignore", "prettier-ignore-start",
		"prettier-ignore-end", "stylistic-only", "no-transform", "skip-coverage",
		"todo", "fixme", "hack", "NOTE",
	}

	// Section label words
	sectionWords := []string{
		"header", "footer", "nav", "sidebar", "main", "content", "article", "section",
		"asides", "wrapper", "container", "grid", "flex", "card", "modal", "dropdown",
		"tooltip", "popup", "menu", "breadcrumb", "pagination", "carousel", "slider",
	}

	// Version-related word pools
	versionPrefixes := []string{"v", "build", "release", "snapshot", "beta"}
	versionNumbers := []string{"1.0.0", "2.3.4", "3.2.1", "1.5.0", "2.0.0-beta"}
	versionSuffixes := []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu", "vwx"}

	// CSS property pools
	cssProperties := map[string][]string{
		"display": {"block", "inline", "flex", "grid", "inline-block", "none"},
		"margin":  {"0", "auto", "10px", "1rem", "16px", "20px"},
		"padding": {"0", "8px", "12px", "16px", "20px", "1rem"},
		"color":   {"#333", "#666", "#999", "currentColor", "rgba(0,0,0,0.5)"},
		"background-color": {
			"transparent", "#f5f5f5", "#e9ecef", "#ffffff", "rgba(255,255,255,0.8)",
		},
		"font-size":   {"12px", "14px", "16px", "18px", "1rem", "0.875rem"},
		"font-weight": {"normal", "bold", "400", "500", "600", "700"},
		"line-height": {"1", "1.2", "1.4", "1.5", "1.6", "1.8", "2"},
		"text-align":  {"left", "center", "right", "justify"},
		"border":      {"none", "1px solid #ccc", "2px solid #eee", "1px solid rgba(0,0,0,0.1)"},
		"width":       {"auto", "100%", "50%", "200px", "min-content", "max-content"},
		"height":      {"auto", "100%", "50%", "200px", "min-content"},
		"opacity":     {"0", "0.1", "0.5", "0.75", "1"},
		"overflow":    {"visible", "hidden", "auto", "scroll", "clip"},
		"position":    {"static", "relative", "absolute", "fixed", "sticky"},
		"z-index":     {"0", "1", "10", "100", "auto"},
	}

	// Property names for CSS rules (must all exist in cssProperties map)
	cssPropertyNames := []string{
		"display", "margin", "padding", "color", "background-color", "font-size",
		"font-weight", "line-height", "text-align", "border", "width", "height",
		"opacity", "overflow", "position", "z-index",
	}

	// CSS selector patterns
	classPrefixes := []string{"xkq", "decoy", "fake", "dummy", "placeholder", "unused", "temp", "extra", "aux", "bonus"}
	classSuffixes := []string{"1a", "2b", "3c", "4d", "5e", "6f", "7g", "8h", "9i", "10j"}
	idPrefixes := []string{"decoy", "unused", "temp", "auxiliary", "bonus", "extra", "decoration"}
	idSuffixes := []string{"a1", "b2", "c3", "d4", "e5", "f6", "g7", "h8", "i9", "j10"}
	pseudoClasses := []string{":hover", ":focus", ":active", ":visited", ":first-child", ":last-child", ":nth-child(2n)", ":not(.existing)"}

	// Meta tag options
	generatorNames := []string{
		"WordPress", "Drupal", "Wix", "Squarespace", "Joomla", "Magento", "Shopify",
		"Ghost", "Hugo", "Next.js", "Nuxt.js", "Gatsby", "VuePress", "Pelican",
		"Grav", "Hugo", "Zola", "Docusaurus", "Astro", "Statamic",
	}
	viewportContents := []string{
		`width=device-width, initial-scale=1.0`,
		`width=device-width, initial-scale=1.0, maximum-scale=1.0`,
		`width=device-width, initial-scale=1.0, viewport-fit=cover`,
		`width=device-width, initial-scale=1.0, user-scalable=no`,
		`width=device-width, initial-scale=1.0, maximum-scale=5.0`,
	}
	robotsContents := []string{
		"index, follow",
		"noindex, nofollow",
		"index, nofollow",
		"noindex, follow",
		"index, follow, max-image-preview:large",
		"index, follow, max-snippet:10, max-video-preview:5",
	}
	themeColors := []string{"#007bff", "#6c757d", "#28a745", "#dc3545", "#ffc107", "#17a2b8", "#6f42c1", "#fd7e14"}
	xUaCompatibleValues := []string{"IE=edge", "IE=11", "IE=10", "IE=9", "IE=8", "IE=edge, chrome=1"}

	// Track injection counts
	htmlCommentsInjected := 0
	hiddenElementsInjected := 0
	unusedCSSRulesInjected := 0
	jsDecoysInjected := 0
	metaTagsInjected := 0

	// --- META TAG INJECTION (3-8 per build) ---
	// Do this first before comments/elements shift positions
	metaTagsToInject := 3 + rd.Intn(6) // 3-8 meta tags
	var metaTags []string
	for i := 0; i < metaTagsToInject; i++ {
		metaTag := generateMetaTag(rd, generatorNames, viewportContents, robotsContents, themeColors, xUaCompatibleValues)
		metaTags = append(metaTags, metaTag)
		metaTagsInjected++
	}
	if len(metaTags) > 0 {
		metaBlock := "\n" + strings.Join(metaTags, "\n") + "\n"
		metaInsertionPoint := findMetaInsertionPoint(html)
		html = html[:metaInsertionPoint] + metaBlock + html[metaInsertionPoint:]
	}

	// --- HTML COMMENT INJECTION (5-20 per build) ---
	// Find valid insertion points (between > and <) and insert comments there
	commentsToInject := 5 + rd.Intn(16) // 5-20 comments
	var comments []string
	for i := 0; i < commentsToInject; i++ {
		comment := generateComment(rd, todoPrefixes, todoTopics, developerNames, lintMarkers, sectionWords, versionPrefixes, versionNumbers, versionSuffixes)
		comments = append(comments, "<!-- "+comment+" -->")
		htmlCommentsInjected++
	}
	// Insert comments at valid positions (between elements), working backwards
	// to avoid position shifting
	insertionPoints := findInsertionPoints(html)
	if len(insertionPoints) > 0 {
		commentPositions := pickRandomIndices(len(insertionPoints), len(comments), rd)
		// Sort descending to insert from end to start (avoids position shifting)
		sort.Sort(sort.Reverse(sort.IntSlice(commentPositions)))
		for i, idx := range commentPositions {
			pos := insertionPoints[idx]
			html = html[:pos] + comments[i] + html[pos:]
		}
	}

	// --- HIDDEN DOM ELEMENT INJECTION (3-10 per build) ---
	hiddenElementsToInject := 3 + rd.Intn(8) // 3-10 elements
	var hiddenElements []string
	for i := 0; i < hiddenElementsToInject; i++ {
		element := generateHiddenElement(rd)
		hiddenElements = append(hiddenElements, element)
		hiddenElementsInjected++
	}
	// Re-find insertion points (HTML has changed from comments/meta)
	insertionPoints = findInsertionPoints(html)
	if len(insertionPoints) > 0 {
		elementPositions := pickRandomIndices(len(insertionPoints), len(hiddenElements), rd)
		sort.Sort(sort.Reverse(sort.IntSlice(elementPositions)))
		for i, idx := range elementPositions {
			pos := insertionPoints[idx]
			html = html[:pos] + hiddenElements[i] + html[pos:]
		}
	}

	// --- CSS UNUSED RULES INJECTION (10-30 per build) ---
	cssRulesToInject := 10 + rd.Intn(21) // 10-30 rules

	var cssBuilder strings.Builder
	cssBuilder.WriteString(css)
	for i := 0; i < cssRulesToInject; i++ {
		rule := generateUnusedCSSRule(rd, cssProperties, cssPropertyNames, classPrefixes, classSuffixes, idPrefixes, idSuffixes, pseudoClasses)
		cssBuilder.WriteString("\n")
		cssBuilder.WriteString(rule)
		unusedCSSRulesInjected++
	}
	css = cssBuilder.String()

	// --- JAVASCRIPT DECOYS INJECTION (3-10 per build) ---
	jsDecoysToInject := 3 + rd.Intn(8) // 3-10 decoys

	var jsDecoys []string
	for i := 0; i < jsDecoysToInject; i++ {
		decoy := generateJSDecoy(rd)
		jsDecoys = append(jsDecoys, decoy)
		jsDecoysInjected++
	}
	// Prepend some, append the rest
	if len(jsDecoys) > 0 {
		// First decoy may be prepended (50% chance)
		prependCount := 0
		if rd.Intn(2) == 0 && len(jsDecoys) > 1 {
			prependCount = 1 + rd.Intn(len(jsDecoys)/2+1)
			if prependCount > len(jsDecoys) {
				prependCount = len(jsDecoys)
			}
		}
		prepended := strings.Join(jsDecoys[:prependCount], "\n")
		appended := strings.Join(jsDecoys[prependCount:], "\n")
		if prepended != "" {
			js = prepended + "\n" + js
		}
		if appended != "" {
			js = js + "\n" + appended
		}
	}

	// Add final manifest fields
	manifest["strategy"] = "live"
	manifest["html_comments_injected"] = htmlCommentsInjected
	manifest["hidden_elements_injected"] = hiddenElementsInjected
	manifest["unused_css_rules_injected"] = unusedCSSRulesInjected
	manifest["js_decoys_injected"] = jsDecoysInjected
	manifest["meta_tags_injected"] = metaTagsInjected

	return html, css, js, manifest, nil
}

// generateComment generates a random comment text from word pools.
func generateComment(rd *rand.Rand, todoPrefixes, todoTopics, developerNames, lintMarkers, sectionWords, versionPrefixes, versionNumbers, versionSuffixes []string) string {
	commentType := rd.Intn(5) // 0=TODO, 1=Version, 2=Developer, 3=Lint, 4=Section

	switch commentType {
	case 0: // TODO
		prefix := todoPrefixes[rd.Intn(len(todoPrefixes))]
		topic := todoTopics[rd.Intn(len(todoTopics))]
		return fmt.Sprintf("TODO: %s %s", prefix, topic)
	case 1: // Version
		prefix := versionPrefixes[rd.Intn(len(versionPrefixes))]
		number := versionNumbers[rd.Intn(len(versionNumbers))]
		suffix := versionSuffixes[rd.Intn(len(versionSuffixes))]
		return fmt.Sprintf("%s %s-%s", prefix, number, suffix)
	case 2: // Developer
		name := developerNames[rd.Intn(len(developerNames))]
		role := rd.Intn(2) == 0
		if role {
			return fmt.Sprintf("author: %s", name)
		}
		return fmt.Sprintf("reviewed by: %s", name)
	case 3: // Lint marker
		marker := lintMarkers[rd.Intn(len(lintMarkers))]
		return marker
	default: // Section
		word := sectionWords[rd.Intn(len(sectionWords))]
		return fmt.Sprintf("%s section", word)
	}
}

// generateHiddenElement generates a random hidden HTML element.
func generateHiddenElement(rd *rand.Rand) string {
	hidingTechnique := rd.Intn(5) // 0-4

	// Content options for hidden elements
	textOptions := []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.",
		"Duis aute irure dolor in reprehenderit in voluptate velit esse.",
		"Cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat.",
	}
	linkOptions := []string{
		"Learn more",
		"Read more",
		"View details",
		"Click here",
		"Discover more",
	}
	listItems := []string{"First item", "Second item", "Third item", "Fourth item", "Fifth item"}

	// Generate content based on element type
	contentType := rd.Intn(3) // 0=paragraph, 1=link, 2=list
	var content string

	switch contentType {
	case 0:
		content = textOptions[rd.Intn(len(textOptions))]
	case 1:
		linkText := linkOptions[rd.Intn(len(linkOptions))]
		content = fmt.Sprintf("<a href=\"#\">%s</a>", linkText)
	default:
		listItemsSample := listItems
		if len(listItems) > 3 && rd.Intn(2) == 0 {
			listItemsSample = listItems[:3+rd.Intn(3)]
		}
		var items string
		for _, item := range listItemsSample {
			items += fmt.Sprintf("<li>%s</li>\n", item)
		}
		content = fmt.Sprintf("<ul>\n%s</ul>", items)
	}

	var style string
	var ariaHidden bool

	switch hidingTechnique {
	case 0: // display:none
		style = `style="display:none"`
	case 1: // visibility:hidden
		style = `style="visibility:hidden"`
	case 2: // off-screen
		style = `style="position:absolute;left:-9999px"`
	case 3: // zero-size
		style = `style="width:0;height:0;overflow:hidden"`
		ariaHidden = true
	default: // transparent
		style = `style="opacity:0;pointer-events:none"`
	}

	// Generate class/ID for the element
	classOrID := ""
	if rd.Intn(2) == 0 {
		classOrID = fmt.Sprintf(`id="decoy_%d"`, rd.Intn(1000))
	} else {
		classOrID = fmt.Sprintf(`class="decoy-class-%d"`, rd.Intn(100))
	}

	// Build the element
	tagName := []string{"div", "span", "section", "aside"}[rd.Intn(4)]
	var builder strings.Builder
	builder.WriteString("<")
	builder.WriteString(tagName)
	builder.WriteString(" ")
	builder.WriteString(classOrID)
	if style != "" {
		builder.WriteString(" ")
		builder.WriteString(style)
	}
	if ariaHidden {
		builder.WriteString(" aria-hidden=\"true\"")
	}
	builder.WriteString(">")
	builder.WriteString(content)
	builder.WriteString("</")
	builder.WriteString(tagName)
	builder.WriteString(">")

	return builder.String()
}

// generateUnusedCSSRule generates a CSS rule with a non-existent selector.
func generateUnusedCSSRule(rd *rand.Rand, cssProperties map[string][]string, cssPropertyNames, classPrefixes, classSuffixes, idPrefixes, idSuffixes, pseudoClasses []string) string {
	selectorType := rd.Intn(4) // 0=class, 1=id, 2=compound, 3=pseudo

	var selector string

	switch selectorType {
	case 0: // Class selector
		prefix := classPrefixes[rd.Intn(len(classPrefixes))]
		suffix := classSuffixes[rd.Intn(len(classSuffixes))]
		selector = fmt.Sprintf(".%s_unused_%s", prefix, suffix)
	case 1: // ID selector
		prefix := idPrefixes[rd.Intn(len(idPrefixes))]
		suffix := idSuffixes[rd.Intn(len(idSuffixes))]
		selector = fmt.Sprintf("#%s_%s_m%d", prefix, suffix, rd.Intn(100))
	case 2: // Compound selector
		parentPrefix := classPrefixes[rd.Intn(len(classPrefixes))]
		parentSuffix := classSuffixes[rd.Intn(len(classSuffixes))]
		childPrefix := classPrefixes[rd.Intn(len(classPrefixes))]
		childSuffix := classSuffixes[rd.Intn(len(classSuffixes))]
		selector = fmt.Sprintf(".%s_%s_wrapper > .%s_%s_child", parentPrefix, parentSuffix, childPrefix, childSuffix)
	default: // Pseudo-class selector
		baseClass := fmt.Sprintf(".fake_%d", rd.Intn(100))
		pseudo := pseudoClasses[rd.Intn(len(pseudoClasses))]
		selector = baseClass + pseudo
	}

	// Generate 1-4 CSS properties
	numProperties := 1 + rd.Intn(4)
	var properties []string
	usedProperties := make(map[string]bool)

	for i := 0; i < numProperties; i++ {
		propName := cssPropertyNames[rd.Intn(len(cssPropertyNames))]
		if usedProperties[propName] {
			continue
		}
		usedProperties[propName] = true

		values := cssProperties[propName]
		value := values[rd.Intn(len(values))]
		properties = append(properties, fmt.Sprintf("    %s: %s;", propName, value))
	}

	// Build the rule
	var builder strings.Builder
	builder.WriteString(selector)
	builder.WriteString(" {\n")
	for _, prop := range properties {
		builder.WriteString(prop)
		builder.WriteString("\n")
	}
	builder.WriteString("}")

	return builder.String()
}

// generateJSDecoy generates a JavaScript statement that produces no visible effect.
func generateJSDecoy(rd *rand.Rand) string {
	decoyType := rd.Intn(5) // 0=unused variable, 1=empty function, 2=false condition, 3=no-op expression, 4=try/catch

	switch decoyType {
	case 0: // Unused variable
		varType := []string{"var", "let", "const"}[rd.Intn(3)]
		varName := fmt.Sprintf("_dk%d", rd.Intn(100))
		valueType := rd.Intn(3) // 0=string, 1=number, 2=object
		switch valueType {
		case 0:
			return fmt.Sprintf("%s %s = \"placeholder_%d\";", varType, varName, rd.Intn(1000))
		case 1:
			return fmt.Sprintf("%s %s = %d;", varType, varName, rd.Intn(1000))
		default:
			return fmt.Sprintf("%s %s = { key: \"value_%d\", count: %d };", varType, varName, rd.Intn(100), rd.Intn(100))
		}
	case 1: // Empty function
		funcName := fmt.Sprintf("_noop_x%d", rd.Intn(100))
		return fmt.Sprintf("function %s() {}", funcName)
	case 2: // Dead code behind false condition
		traceNum := rd.Intn(100)
		return fmt.Sprintf("if (false) { console.debug(\"trace-%03d\"); }", traceNum)
	case 3: // No-op expression
		exprType := rd.Intn(3)
		switch exprType {
		case 0:
			return "void 0;"
		case 1:
			return "void (0);"
		default:
			return "void function() {}();"
		}
	default: // Try/catch
		return "try { void 0; } catch(e) {}"
	}
}

// generateMetaTag generates a random meta tag.
func generateMetaTag(rd *rand.Rand, generatorNames, viewportContents, robotsContents, themeColors, xUaCompatibleValues []string) string {
	metaType := rd.Intn(6) // 0=generator, 1=viewport, 2=robots, 3=theme-color, 4=X-UA-Compatible, 5=format-detection

	var name, content string

	switch metaType {
	case 0: // Generator
		name = "generator"
		content = generatorNames[rd.Intn(len(generatorNames))]
	case 1: // Viewport
		name = "viewport"
		content = viewportContents[rd.Intn(len(viewportContents))]
	case 2: // Robots
		name = "robots"
		content = robotsContents[rd.Intn(len(robotsContents))]
	case 3: // Theme color
		name = "theme-color"
		content = themeColors[rd.Intn(len(themeColors))]
	case 4: // X-UA-Compatible
		name = "http-equiv"
		content = fmt.Sprintf(`name="X-UA-Compatible" content="%s"`, xUaCompatibleValues[rd.Intn(len(xUaCompatibleValues))])
		return fmt.Sprintf(`<meta %s>`, content)
	default: // Format detection
		name = "format-detection"
		content = "telephone=no"
	}

	// Build meta tag
	var builder strings.Builder
	builder.WriteString("<meta")
	builder.WriteString(" name=\"")
	builder.WriteString(name)
	builder.WriteString("\" content=\"")
	builder.WriteString(content)
	builder.WriteString("\">")

	return builder.String()
}

// findInsertionPoints returns valid positions for inserting elements in HTML.
// Valid positions are after > and before <.
func findInsertionPoints(html string) []int {
	var points []int
	i := 0
	n := len(html)

	for i < n {
		if html[i] == '>' {
			// Look ahead to find next < (not inside a comment or string)
			j := i + 1
			for j < n {
				if html[j] == '<' {
					// Check if we're inside a comment or string
					if j+3 <= n && html[j:j+4] == "<!--" {
						// Skip to end of comment
						endComment := strings.Index(html[j:], "-->")
						if endComment != -1 {
							j = j + endComment + 3
							continue
						}
					}
					if j+1 <= n && html[j] == '<' {
						points = append(points, j)
					}
					break
				}
				j++
			}
		}
		i++
	}

	return points
}

// findMetaInsertionPoint returns the position to insert meta tags.
func findMetaInsertionPoint(html string) int {
	// Look for head section
	headEnd := strings.Index(html, "</head>")
	if headEnd != -1 {
		return headEnd
	}

	// Look for body start
	bodyStart := strings.Index(html, "<body")
	if bodyStart != -1 {
		return bodyStart
	}

	// Fall back to after doctype
	doctypePos := strings.Index(html, "<!DOCTYPE")
	if doctypePos != -1 {
		return doctypePos
	}

	return 0
}

// pickRandomIndices picks up to n unique random indices in the range [0, max).
// Returns sorted indices. If n >= max, returns all indices.
func pickRandomIndices(max, n int, rd *rand.Rand) []int {
	if max <= 0 {
		return nil
	}
	if n >= max {
		indices := make([]int, max)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}
	used := make(map[int]bool, n)
	for len(used) < n {
		used[rd.Intn(max)] = true
	}
	indices := make([]int, 0, len(used))
	for idx := range used {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}

// generateUniquePositions generates N unique positions for injections.
func generateUniquePositions(maxPos, n int, rd *rand.Rand) []int {
	if n >= maxPos {
		// Generate all positions
		positions := make([]int, maxPos)
		for i := 0; i < maxPos; i++ {
			positions[i] = i
		}
		return positions
	}

	positions := make([]int, 0, n)
	used := make(map[int]bool)

	for len(positions) < n {
		pos := rd.Intn(maxPos)
		if !used[pos] {
			positions = append(positions, pos)
			used[pos] = true
		}
	}

	sort.Ints(positions)
	return positions
}

// insertStringAt inserts a string at the given position.
func insertStringAt(s, insert string, pos int) string {
	if pos >= len(s) {
		return s + insert
	}
	if pos <= 0 {
		return insert + s
	}
	return s[:pos] + insert + s[pos:]
}
