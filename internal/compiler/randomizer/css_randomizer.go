// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"math/rand"
	"regexp"
	"strings"
)

// LiveCSSRandomizer randomizes CSS class names to prevent fingerprinting.
type LiveCSSRandomizer struct{}

// NewLiveCSSRandomizer creates a new CSS class name randomizer.
func NewLiveCSSRandomizer() *LiveCSSRandomizer {
	return &LiveCSSRandomizer{}
}

// NamingConvention represents the style used for generating class names.
type NamingConvention int

const (
	NamingLowercase NamingConvention = iota
	NamingCamelCase
	NamingHyphenated
	NamingUnderscore
	NamingBemLike
)

// namingConventions maps naming convention indices to their names.
var namingConventionNames = map[NamingConvention]string{
	NamingLowercase:  "lowercase",
	NamingCamelCase:  "camelCase",
	NamingHyphenated: "hyphenated",
	NamingUnderscore: "underscore",
	NamingBemLike:    "bem-like",
}

// RandomizeCSS replaces CSS class names in both HTML and CSS content.
// Returns the randomized HTML, randomized CSS, and a manifest of mappings.
//
// The randomization includes:
//   - Random selection of naming convention per build
//   - Generation of unique class names (4-12 characters)
//   - Replacement of class attributes in HTML
//   - Replacement of class selectors in CSS
//   - Preservation of class names inside <script> tags
//   - Support for compound selectors (.foo.bar, .foo .bar, etc.)
func (r *LiveCSSRandomizer) RandomizeCSS(html string, css string, seed int64) (string, string, map[string]any, error) {
	// Create seeded random source for deterministic results
	src := rand.NewSource(seed)
	rd := rand.New(src)

	// Manifest to track all randomization decisions
	manifest := map[string]any{}

	// Select naming convention for this build
	convention := r.selectNamingConvention(rd)
	manifest["naming_convention"] = namingConventionNames[convention]

	// Extract all class names from HTML and CSS
	classNames := r.extractClassNames(html, css)
	manifest["classes_extracted"] = len(classNames)

	// Generate random names for each class
	classMapping := r.generateClassMapping(classNames, convention, rd)
	manifest["classes_mapped"] = len(classMapping)

	// Randomize HTML class attributes
	randomizedHTML, htmlReplacements := r.randomizeHTML(html, classMapping, rd)
	manifest["html_replacements"] = htmlReplacements

	// Randomize CSS selectors inside <style> blocks embedded in the HTML
	randomizedHTML, styleReplacements := r.randomizeEmbeddedStyles(randomizedHTML, classMapping, rd)
	manifest["embedded_style_replacements"] = styleReplacements

	// Randomize standalone CSS selectors (uses same old→new mapping)
	randomizedCSS, cssReplacements := r.randomizeCSS(css, classMapping, rd)
	manifest["css_replacements"] = cssReplacements

	// Add final mapping to manifest
	manifest["mapping"] = classMapping
	manifest["strategy"] = "live"

	return randomizedHTML, randomizedCSS, manifest, nil
}

// selectNamingConvention selects a naming convention based on the random source.
func (r *LiveCSSRandomizer) selectNamingConvention(rd *rand.Rand) NamingConvention {
	return NamingConvention(rd.Intn(int(NamingBemLike + 1)))
}

// extractClassNames extracts all unique class names from HTML and CSS content.
func (r *LiveCSSRandomizer) extractClassNames(html, css string) []string {
	classes := make(map[string]bool)

	// Extract from HTML class attributes
	// Use a simple pattern to extract class attribute values
	// Pattern: class\s*=\s*(["'])([^"']*?)["']
	// Note: Using ["'] for closing quote instead of \1 backreference to avoid RE2 issues
	htmlClassRegex := regexp.MustCompile(`class\s*=\s*(["'])([^"']*?)["']`)
	for _, match := range htmlClassRegex.FindAllStringSubmatch(html, -1) {
		// match[0] = full match (class="value" or class='value')
		// match[1] = opening quote character
		// match[2] = class value (without quotes)
		classValue := match[2]
		// Split by whitespace to handle multi-class attributes
		for _, className := range strings.Fields(classValue) {
			// Remove any leading/trailing non-alphanumeric characters
			className = strings.Trim(className, ". \t\r\n")
			if className != "" && r.isValidCSSIdentifier(className) {
				classes[className] = true
			}
		}
	}

	// Extract from CSS class selectors in standalone CSS
	cssClassRegex := regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_-]*)`)
	for _, match := range cssClassRegex.FindAllStringSubmatch(css, -1) {
		className := match[1]
		if className != "" && r.isValidCSSIdentifier(className) {
			classes[className] = true
		}
	}

	// Extract from CSS class selectors in embedded <style> blocks in HTML
	for _, styleContent := range r.extractStyleBlocks(html) {
		for _, match := range cssClassRegex.FindAllStringSubmatch(styleContent, -1) {
			className := match[1]
			if className != "" && r.isValidCSSIdentifier(className) {
				classes[className] = true
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(classes))
	for className := range classes {
		result = append(result, className)
	}

	return result
}

// isValidCSSIdentifier checks if a string is a valid CSS identifier.
func (r *LiveCSSRandomizer) isValidCSSIdentifier(name string) bool {
	if name == "" {
		return false
	}

	// Must start with letter, underscore, or hyphen
	firstChar := name[0]
	if !(firstChar >= 'a' && firstChar <= 'z' ||
		firstChar >= 'A' && firstChar <= 'Z' ||
		firstChar == '_' || firstChar == '-') {
		return false
	}

	// All characters must be valid for CSS identifiers
	// Letters, digits, hyphens, underscores, and (in the middle) can start with hyphen
	for i, c := range name {
		if c == '-' || c == '_' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			continue
		}
		if c >= '0' && c <= '9' && i > 0 {
			continue
		}
		return false
	}

	return true
}

// generateClassMapping generates random names for each original class.
func (r *LiveCSSRandomizer) generateClassMapping(classes []string, convention NamingConvention, rd *rand.Rand) map[string]string {
	mapping := make(map[string]string, len(classes))
	usedNames := make(map[string]bool, len(classes))

	for _, className := range classes {
		newName := r.generateRandomName(convention, rd, usedNames)
		mapping[className] = newName
	}

	return mapping
}

// generateRandomName generates a single random class name.
func (r *LiveCSSRandomizer) generateRandomName(convention NamingConvention, rd *rand.Rand, usedNames map[string]bool) string {
	var name string

	// Class name length: 4-12 characters
	length := 4 + rd.Intn(9) // 4 to 12 inclusive

	for {
		name = r.generateNameWithConvention(convention, length, rd)
		if !usedNames[name] {
			break
		}
		// Try again with same or slightly different length
		if rd.Intn(3) == 0 {
			length = 4 + rd.Intn(9)
		}
	}

	usedNames[name] = true
	return name
}

// generateNameWithConvention generates a class name using the specified convention.
func (r *LiveCSSRandomizer) generateNameWithConvention(convention NamingConvention, length int, rd *rand.Rand) string {
	switch convention {
	case NamingLowercase:
		return r.generateLowercase(length, rd)
	case NamingCamelCase:
		return r.generateCamelCase(length, rd)
	case NamingHyphenated:
		return r.generateHyphenated(length, rd)
	case NamingUnderscore:
		return r.generateUnderscore(length, rd)
	case NamingBemLike:
		return r.generateBemLike(length, rd)
	default:
		return r.generateLowercase(length, rd)
	}
}

// generateLowercase generates a simple lowercase name (e.g., "abcdef").
func (r *LiveCSSRandomizer) generateLowercase(length int, rd *rand.Rand) string {
	name := make([]byte, length)
	for i := 0; i < length; i++ {
		name[i] = byte('a' + rd.Intn(26))
	}
	return string(name)
}

// generateCamelCase generates a camelCase name (e.g., "abcDef").
func (r *LiveCSSRandomizer) generateCamelCase(length int, rd *rand.Rand) string {
	if length < 2 {
		// Single character, make it lowercase
		return string('a' + byte(rd.Intn(26)))
	}

	name := make([]byte, length)
	// First character is always lowercase
	name[0] = byte('a' + rd.Intn(26))

	// Determine break points for uppercase letters
	// We'll have roughly 1 uppercase every 3-4 characters
	breakModulus := 3 + rd.Intn(2)
	upperStart := 1 + rd.Intn(2) // Start uppercase breaks after first few chars

	for i := 1; i < length; i++ {
		if i >= upperStart && (i-breakModulus)%breakModulus == 0 {
			// Start of a new camelCase segment - uppercase
			name[i] = byte('A' + rd.Intn(26))
		} else {
			name[i] = byte('a' + rd.Intn(26))
		}
	}

	return string(name)
}

// generateHyphenated generates a hyphenated name (e.g., "abc-def").
func (r *LiveCSSRandomizer) generateHyphenated(length int, rd *rand.Rand) string {
	var sb strings.Builder
	segmentLength := 3 + rd.Intn(3) // 3-5 characters per segment
	segments := length / segmentLength
	if segments < 1 {
		segments = 1
	}

	for i := 0; i < segments; i++ {
		if i > 0 {
			sb.WriteByte('-')
		}
		segLen := segmentLength
		if i == segments-1 {
			segLen = length - sb.Len()
		}
		for j := 0; j < segLen && sb.Len() < length; j++ {
			sb.WriteByte('a' + byte(rd.Intn(26)))
		}
	}

	return sb.String()
}

// generateUnderscore generates an underscored name (e.g., "abc_def").
func (r *LiveCSSRandomizer) generateUnderscore(length int, rd *rand.Rand) string {
	var sb strings.Builder
	segmentLength := 3 + rd.Intn(3) // 3-5 characters per segment
	segments := length / segmentLength
	if segments < 1 {
		segments = 1
	}

	for i := 0; i < segments; i++ {
		if i > 0 {
			sb.WriteByte('_')
		}
		segLen := segmentLength
		if i == segments-1 {
			segLen = length - sb.Len()
		}
		for j := 0; j < segLen && sb.Len() < length; j++ {
			sb.WriteByte('a' + byte(rd.Intn(26)))
		}
	}

	return sb.String()
}

// generateBemLike generates a BEM-like name (e.g., "block__element--modifier").
func (r *LiveCSSRandomizer) generateBemLike(length int, rd *rand.Rand) string {
	var sb strings.Builder

	// Generate 1-3 segments
	numSegments := 1 + rd.Intn(3)
	baseLength := length / numSegments

	for i := 0; i < numSegments; i++ {
		if i > 0 {
			// Add BEM separator
			if rd.Intn(2) == 0 {
				sb.WriteString("__") // element separator
			} else {
				sb.WriteString("--") // modifier separator
			}
		}
		segLen := baseLength + rd.Intn(3)
		for j := 0; j < segLen && sb.Len() < length; j++ {
			sb.WriteByte('a' + byte(rd.Intn(26)))
		}
	}

	return sb.String()
}

// randomizeHTML replaces class names in HTML class attributes.
func (r *LiveCSSRandomizer) randomizeHTML(html string, mapping map[string]string, rd *rand.Rand) (string, int) {
	replacements := 0

	// Pattern to match class attributes, preserving script content
	// We'll process HTML in chunks, skipping <script> tag contents

	// First, identify script tag ranges to skip
	scriptRanges := r.findScriptRanges(html)

	// Function to check if a position is inside a script tag
	isInScript := func(pos int) bool {
		for _, rng := range scriptRanges {
			if pos >= rng[0] && pos < rng[1] {
				return true
			}
		}
		return false
	}

	// Pattern to match class attribute values
	// Using ["'] for closing quote (simpler than \g{name} which RE2 doesn't support well)
	classAttrRegex := regexp.MustCompile(`class\s*=\s*(["'])([^"']*?)["']`)

	// We need to build the result incrementally
	var sb strings.Builder
	lastEnd := 0

	// Use FindAllStringSubmatchIndex for precise index-based matching
	// Pattern: class\s*=\s*(["'])([^"']*?)["']
	// Each match: [fullStart, fullEnd, quoteStart, quoteEnd, classValueStart, classValueEnd]
	for _, match := range classAttrRegex.FindAllStringSubmatchIndex(html, -1) {
		start, end := match[0], match[1]

		// Skip if inside a script tag
		if isInScript(start) {
			continue
		}

		// Write content before this match
		sb.WriteString(html[lastEnd:start])

		// Extract quote and class value
		// match[2] = opening quote start, match[3] = opening quote end
		// match[4] = class value start, match[5] = class value end
		quoteStart, quoteEnd := match[2], match[3]
		classStart, classEnd := match[4], match[5]

		// Get the quote character (opening quote)
		quote := html[quoteStart:quoteEnd]

		// Get the class value (between quotes)
		classValue := html[classStart:classEnd]

		// Replace class names in the value
		newClassValue, count := r.replaceClassNames(classValue, mapping, rd)
		replacements += count

		// Reconstruct the attribute: class="newValue" or class='newValue'
		sb.WriteString(html[start:quoteStart]) // "class="
		sb.WriteString(quote)                  // opening quote
		sb.WriteString(newClassValue)          // new class names
		sb.WriteString(quote)                  // closing quote (same type)

		lastEnd = end
	}

	// Write remaining content
	sb.WriteString(html[lastEnd:])

	return sb.String(), replacements
}

// findScriptRanges returns the start/end positions of all <script> tags.
func (r *LiveCSSRandomizer) findScriptRanges(html string) [][2]int {
	var ranges [][2]int
	i := 0
	n := len(html)

	for i < n {
		// Find opening script tag
		if strings.HasPrefix(html[i:], "<script") {
			start := i
			// Find end of opening tag
			i += 7 // len("<script")
			for i < n && html[i] != '>' {
				i++
			}
			i++ // past '>'

			// Check for self-closing
			if strings.HasPrefix(html[i-2:], "/>") {
				ranges = append(ranges, [2]int{start, i})
				continue
			}

			// Find closing script tag
			endTagPos := strings.Index(html[i:], "</script>")
			if endTagPos != -1 {
				i += endTagPos + 9 // len("</script>") = 9, now past '>'
				ranges = append(ranges, [2]int{start, i})
			}
		} else {
			i++
		}
	}

	return ranges
}

// replaceClassNames replaces class names in a class attribute value.
func (r *LiveCSSRandomizer) replaceClassNames(classValue string, mapping map[string]string, rd *rand.Rand) (string, int) {
	replacements := 0

	// Split by whitespace
	classes := strings.Fields(classValue)
	result := make([]string, len(classes))

	for i, className := range classes {
		// Remove any leading/trailing dots (from compound selectors in class attr)
		className = strings.TrimPrefix(className, ".")

		if newName, ok := mapping[className]; ok {
			result[i] = newName
			replacements++
		} else {
			result[i] = className
		}
	}

	return strings.Join(result, " "), replacements
}

// randomizeCSS replaces class selectors in CSS.
func (r *LiveCSSRandomizer) randomizeCSS(css string, mapping map[string]string, rd *rand.Rand) (string, int) {
	replacements := 0

	// Pattern to match CSS class selectors
	// This handles .class, .class:hover, .class1.class2, etc.
	classSelectorRegex := regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_-]*)`)

	var sb strings.Builder
	lastEnd := 0

	for _, match := range classSelectorRegex.FindAllStringSubmatchIndex(css, -1) {
		start, end := match[0], match[1]

		// Get the full match including the dot
		fullMatch := css[start:end]

		// Extract class name (without the dot)
		className := fullMatch[1:] // skip the '.'

		sb.WriteString(css[lastEnd:start])
		if newName, ok := mapping[className]; ok {
			sb.WriteString("." + newName)
			replacements++
		} else {
			sb.WriteString(fullMatch)
		}
		lastEnd = end
	}

	// Write remaining content
	sb.WriteString(css[lastEnd:])

	return sb.String(), replacements
}

// extractStyleBlocks returns the content of all <style>...</style> blocks in HTML.
func (r *LiveCSSRandomizer) extractStyleBlocks(html string) []string {
	var blocks []string
	remaining := html
	for {
		openIdx := strings.Index(strings.ToLower(remaining), "<style")
		if openIdx < 0 {
			break
		}
		// Find end of opening tag.
		gtIdx := strings.Index(remaining[openIdx:], ">")
		if gtIdx < 0 {
			break
		}
		contentStart := openIdx + gtIdx + 1
		closeIdx := strings.Index(strings.ToLower(remaining[contentStart:]), "</style>")
		if closeIdx < 0 {
			break
		}
		blocks = append(blocks, remaining[contentStart:contentStart+closeIdx])
		remaining = remaining[contentStart+closeIdx+8:]
	}
	return blocks
}

// randomizeEmbeddedStyles replaces CSS class selectors inside <style> blocks in HTML.
func (r *LiveCSSRandomizer) randomizeEmbeddedStyles(html string, mapping map[string]string, rd *rand.Rand) (string, int) {
	totalReplacements := 0
	var sb strings.Builder
	remaining := html

	for {
		openIdx := strings.Index(strings.ToLower(remaining), "<style")
		if openIdx < 0 {
			sb.WriteString(remaining)
			break
		}
		// Find end of opening tag.
		gtIdx := strings.Index(remaining[openIdx:], ">")
		if gtIdx < 0 {
			sb.WriteString(remaining)
			break
		}
		contentStart := openIdx + gtIdx + 1
		closeIdx := strings.Index(strings.ToLower(remaining[contentStart:]), "</style>")
		if closeIdx < 0 {
			sb.WriteString(remaining)
			break
		}

		// Write everything up to and including the opening <style...> tag.
		sb.WriteString(remaining[:contentStart])

		// Randomize the CSS content inside this <style> block.
		styleContent := remaining[contentStart : contentStart+closeIdx]
		randomizedCSS, count := r.randomizeCSS(styleContent, mapping, rd)
		totalReplacements += count
		sb.WriteString(randomizedCSS)

		// Advance past </style>.
		remaining = remaining[contentStart+closeIdx:]
	}

	return sb.String(), totalReplacements
}
