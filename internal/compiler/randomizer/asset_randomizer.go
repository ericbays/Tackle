// Package randomizer implements anti-fingerprinting randomization engines.
package randomizer

import (
	"math/rand"
	"regexp"
	"sort"
	"strings"
)

// LiveAssetRandomizer randomizes asset file paths to prevent fingerprinting.
type LiveAssetRandomizer struct{}

// NewLiveAssetRandomizer creates a new asset path randomizer.
func NewLiveAssetRandomizer() *LiveAssetRandomizer {
	return &LiveAssetRandomizer{}
}

// RandomizeAssets renames asset files and updates references in HTML and CSS.
// Returns the updated files map, updated HTML, updated CSS, and a manifest.
//
// The randomization includes:
//   - Random root directory name (3-8 lowercase alphanumeric chars)
//   - Random subdirectory depth (0-3 levels per file)
//   - Random subdirectory names (2-6 lowercase alphanumeric chars each)
//   - Random filenames (4-12 lowercase alphanumeric chars)
//   - Preservation of file extensions
//   - Replacement of file references in HTML (src, href, srcset)
//   - Replacement of file references in CSS (url(), @import)
//   - External URL preservation (http://, https://, //, data:)
//   - Script body preservation (JavaScript string literals)
func (r *LiveAssetRandomizer) RandomizeAssets(files map[string][]byte, html string, css string, seed int64) (map[string][]byte, string, string, map[string]any, error) {
	// Create seeded random source for deterministic results
	src := rand.NewSource(seed)
	rd := rand.New(src)

	// Manifest to track all randomization decisions
	manifest := map[string]any{}

	// Generate root directory name (3-8 lowercase alphanumeric chars)
	rootDir := generateRandomName(rd, 3, 8)
	manifest["root_dir"] = rootDir

	// Sort file paths for deterministic processing
	filePaths := make([]string, 0, len(files))
	for path := range files {
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)

	// Generate mapping of original path -> new randomized path
	mapping := make(map[string]string, len(files))
	for _, originalPath := range filePaths {
		newPath := r.generateRandomPath(originalPath, rootDir, rd)
		mapping[originalPath] = newPath
	}
	manifest["mapping"] = mapping
	manifest["files_mapped"] = len(mapping)

	// Build new files map with randomized paths
	newFiles := make(map[string][]byte, len(files))
	for originalPath, content := range files {
		newPath := mapping[originalPath]
		newFiles[newPath] = content
	}

	// Replace references in HTML
	htmlReplacements := r.replaceHTMLReferences(&html, mapping, rd)
	manifest["html_replacements"] = htmlReplacements

	// Replace references in CSS
	cssReplacements := r.replaceCSSReferences(&css, mapping, rd)
	manifest["css_replacements"] = cssReplacements

	// Add final manifest fields
	manifest["strategy"] = "live"

	return newFiles, html, css, manifest, nil
}

// generateRandomPath generates a randomized path for the given original path.
func (r *LiveAssetRandomizer) generateRandomPath(originalPath, rootDir string, rd *rand.Rand) string {
	// Extract directory and filename from original path
	lastSlash := strings.LastIndex(originalPath, "/")
	var filename string
	if lastSlash != -1 {
		filename = originalPath[lastSlash+1:]
	} else {
		filename = originalPath
	}

	// Extract extension from filename
	ext := ""
	dotIdx := strings.LastIndex(filename, ".")
	if dotIdx != -1 {
		ext = filename[dotIdx:]
		filename = filename[:dotIdx]
	}

	// Generate subdirectory depth (0-3 levels)
	subDirDepth := rd.Intn(4) // 0, 1, 2, or 3

	// Build subdirectory path
	var subDirPath strings.Builder
	for i := 0; i < subDirDepth; i++ {
		subDirName := generateRandomName(rd, 2, 6)
		subDirPath.WriteString(subDirName)
		subDirPath.WriteString("/")
	}

	// Generate random filename (4-12 chars)
	newBaseName := generateRandomName(rd, 4, 12)

	// Construct the full randomized path
	var sb strings.Builder
	sb.WriteString(rootDir)
	sb.WriteString("/")
	sb.WriteString(subDirPath.String())
	sb.WriteString(newBaseName)
	sb.WriteString(ext)

	return sb.String()
}

// generateRandomName generates a random lowercase alphanumeric string.
func generateRandomName(rd *rand.Rand, minLength, maxLength int) string {
	length := minLength + rd.Intn(maxLength-minLength+1)
	name := make([]byte, length)
	for i := 0; i < length; i++ {
		// Lowercase letters a-z (0-25) and digits 0-9 (26-35)
		val := rd.Intn(36)
		if val < 26 {
			name[i] = byte('a' + val)
		} else {
			name[i] = byte('0' + (val - 26))
		}
	}
	return string(name)
}

// replaceHTMLReferences replaces file references in HTML content.
func (r *LiveAssetRandomizer) replaceHTMLReferences(html *string, mapping map[string]string, rd *rand.Rand) int {
	replacements := 0

	// First, identify script tag ranges to skip
	scriptRanges := r.findScriptBodyRanges(*html)

	// Function to check if a position is inside a script tag
	isInScript := func(pos int) bool {
		for _, rng := range scriptRanges {
			if pos >= rng[0] && pos < rng[1] {
				return true
			}
		}
		return false
	}

	// Match src/href/srcset with double-quoted or single-quoted values
	// We use two patterns since Go regexp doesn't support backreferences
	dqRegex := regexp.MustCompile(`(src|href|srcset)\s*=\s*"([^"]*?)"`)
	sqRegex := regexp.MustCompile(`(src|href|srcset)\s*=\s*'([^']*?)'`)

	type attrMatch struct {
		fullStart, fullEnd int
		attrStart, attrEnd int
		valStart, valEnd   int
		quote              string
	}

	var allMatches []attrMatch

	for _, m := range dqRegex.FindAllStringSubmatchIndex(*html, -1) {
		if len(m) >= 6 {
			allMatches = append(allMatches, attrMatch{m[0], m[1], m[2], m[3], m[4], m[5], `"`})
		}
	}
	for _, m := range sqRegex.FindAllStringSubmatchIndex(*html, -1) {
		if len(m) >= 6 {
			allMatches = append(allMatches, attrMatch{m[0], m[1], m[2], m[3], m[4], m[5], `'`})
		}
	}

	// Sort by position
	sort.Slice(allMatches, func(i, j int) bool {
		return allMatches[i].fullStart < allMatches[j].fullStart
	})

	var sb strings.Builder
	lastEnd := 0

	for _, am := range allMatches {
		fullStart, fullEnd := am.fullStart, am.fullEnd

		// Skip if inside a script body
		if isInScript(fullStart) {
			continue
		}

		attrName := (*html)[am.attrStart:am.attrEnd]
		quote := am.quote
		attrValue := (*html)[am.valStart:am.valEnd]

		// For srcset, handle comma-separated values
		var newValue string
		var count int
		if attrName == "srcset" {
			newValue, count = r.replaceSrcsetValue(attrValue, mapping)
		} else {
			newValue, count = r.replaceSinglePath(attrValue, mapping)
		}
		replacements += count

		// Write content before this match
		sb.WriteString((*html)[lastEnd:fullStart])

		// Write reconstructed attribute
		sb.WriteString(attrName)
		sb.WriteString("=")
		sb.WriteString(quote)
		sb.WriteString(newValue)
		sb.WriteString(quote)

		lastEnd = fullEnd
	}

	// Write remaining content
	sb.WriteString((*html)[lastEnd:])

	*html = sb.String()

	return replacements
}

// replaceSinglePath replaces a single file path if it exists in the mapping.
func (r *LiveAssetRandomizer) replaceSinglePath(path string, mapping map[string]string) (string, int) {
	if isExternalURL(path) {
		return path, 0
	}
	if newPath, ok := mapping[path]; ok {
		return newPath, 1
	}
	return path, 0
}

// replaceSrcsetValue handles comma-separated srcset values like "img1.jpg 1x, img2.jpg 2x".
func (r *LiveAssetRandomizer) replaceSrcsetValue(value string, mapping map[string]string) (string, int) {
	parts := strings.Split(value, ",")
	count := 0
	for i, part := range parts {
		part = strings.TrimSpace(part)
		// srcset entries are "path descriptor" e.g. "img.jpg 2x" or "img.jpg 300w"
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		path := fields[0]
		if newPath, ok := mapping[path]; ok {
			fields[0] = newPath
			count++
		}
		parts[i] = strings.Join(fields, " ")
	}
	return strings.Join(parts, ", "), count
}

// findScriptBodyRanges returns the start/end positions of script tag bodies
// (the content between <script...> and </script>), excluding the tags themselves.
// This allows src attributes on <script> tags to be replaced while preserving
// paths inside JavaScript code.
func (r *LiveAssetRandomizer) findScriptBodyRanges(html string) [][2]int {
	var ranges [][2]int
	i := 0
	n := len(html)

	for i < n {
		// Find opening script tag (case-insensitive match on "<script")
		if i+7 <= n && strings.EqualFold(html[i:i+7], "<script") {
			// Find end of opening tag
			i += 7
			for i < n && html[i] != '>' {
				i++
			}
			i++ // past '>'

			// Check for self-closing (no body)
			if i >= 2 && html[i-2:i] == "/>" {
				continue
			}

			// Body starts here (after the >)
			bodyStart := i

			// Find closing script tag
			endTagPos := strings.Index(html[i:], "</script>")
			if endTagPos != -1 {
				bodyEnd := i + endTagPos
				ranges = append(ranges, [2]int{bodyStart, bodyEnd})
				i = bodyEnd + 9 // past </script>
			}
		} else {
			i++
		}
	}

	return ranges
}

// isExternalURL checks if a path is an external URL.
func isExternalURL(path string) bool {
	path = strings.TrimSpace(path)
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "//") ||
		strings.HasPrefix(path, "data:")
}

// replaceCSSReferences replaces file references in CSS content.
func (r *LiveAssetRandomizer) replaceCSSReferences(css *string, mapping map[string]string, rd *rand.Rand) int {
	replacements := 0

	// Match url() with no quotes, double quotes, or single quotes
	urlNoQuoteRegex := regexp.MustCompile(`url\s*\(\s*([^"'\s)][^)]*?)\s*\)`)
	urlDQRegex := regexp.MustCompile(`url\s*\(\s*"([^"]+)"\s*\)`)
	urlSQRegex := regexp.MustCompile(`url\s*\(\s*'([^']+)'\s*\)`)

	// @import with double or single quotes
	importDQRegex := regexp.MustCompile(`@import\s+"([^"]+)"`)
	importSQRegex := regexp.MustCompile(`@import\s+'([^']+)'`)

	// Collect all matches with their positions
	type cssMatch struct {
		fullStart int
		fullEnd   int
		path      string
		quote     string
		isImport  bool
	}

	var matches []cssMatch

	// Helper to collect url() matches from a regex with 1 capture group (the path)
	collectURL := func(re *regexp.Regexp, quote string) {
		for _, m := range re.FindAllStringSubmatchIndex(*css, -1) {
			if len(m) < 4 {
				continue
			}
			pathStart, pathEnd := m[2], m[3]
			path := (*css)[pathStart:pathEnd]
			matches = append(matches, cssMatch{
				fullStart: m[0],
				fullEnd:   m[1],
				path:      strings.TrimSpace(path),
				quote:     quote,
				isImport:  false,
			})
		}
	}

	collectURL(urlNoQuoteRegex, "")
	collectURL(urlDQRegex, `"`)
	collectURL(urlSQRegex, `'`)

	// Collect @import matches
	collectImport := func(re *regexp.Regexp, quote string) {
		for _, m := range re.FindAllStringSubmatchIndex(*css, -1) {
			if len(m) < 4 {
				continue
			}
			pathStart, pathEnd := m[2], m[3]
			path := (*css)[pathStart:pathEnd]
			matches = append(matches, cssMatch{
				fullStart: m[0],
				fullEnd:   m[1],
				path:      path,
				quote:     quote,
				isImport:  true,
			})
		}
	}

	collectImport(importDQRegex, `"`)
	collectImport(importSQRegex, `'`)

	// Sort matches by position
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].fullStart < matches[j].fullStart
	})

	// Process matches in reverse order to preserve positions
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]

		// Skip external or data URLs
		if isExternalURL(m.path) {
			continue
		}

		// Replace if path is in mapping
		if newPath, ok := mapping[m.path]; ok {
			var replacement string
			if m.isImport {
				replacement = "@import " + m.quote + newPath + m.quote
			} else {
				replacement = "url(" + m.quote + newPath + m.quote + ")"
			}

			*css = (*css)[:m.fullStart] + replacement + (*css)[m.fullEnd:]
			replacements++
		}
	}

	return replacements
}
