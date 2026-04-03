// Package randomizer defines interfaces for anti-fingerprinting randomization engines.
// Implementations are provided by Cline-delegated tasks (AF-1 through AF-6).
// No-op passthrough implementations are provided for development and testing.
package randomizer

// DOMRandomizer randomizes the HTML DOM structure to prevent fingerprinting.
type DOMRandomizer interface {
	// RandomizeDOM applies structural randomization to HTML content.
	// Returns the randomized HTML and a manifest of decisions made.
	RandomizeDOM(html string, seed int64) (string, map[string]any, error)
}

// CSSRandomizer randomizes CSS class names to prevent fingerprinting.
type CSSRandomizer interface {
	// RandomizeCSS replaces CSS class names in both HTML and CSS content.
	// Returns the randomized HTML, randomized CSS, and a manifest of mappings.
	RandomizeCSS(html string, css string, seed int64) (string, string, map[string]any, error)
}

// AssetRandomizer randomizes asset file paths to prevent fingerprinting.
type AssetRandomizer interface {
	// RandomizeAssets renames asset files and updates references in HTML and CSS.
	// Returns the updated files map, updated HTML, updated CSS, and a manifest.
	RandomizeAssets(files map[string][]byte, html string, css string, seed int64) (map[string][]byte, string, string, map[string]any, error)
}

// DecoyInjector injects non-functional decoy content to vary output between builds.
type DecoyInjector interface {
	// InjectDecoys adds decoy HTML comments, hidden elements, unused CSS, and JS no-ops.
	// Returns the modified HTML, CSS, JS, and a manifest of injections.
	InjectDecoys(html string, css string, js string, seed int64) (string, string, string, map[string]any, error)
}

// HeaderRandomizer generates a randomized HTTP response header profile.
type HeaderRandomizer interface {
	// GenerateHeaderProfile produces a set of HTTP headers and Go middleware source code.
	// Returns the header map, the Go source for a middleware function, and a manifest.
	GenerateHeaderProfile(seed int64) (map[string]string, string, map[string]any, error)
}
