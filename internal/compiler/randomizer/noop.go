package randomizer

// NoopDOMRandomizer is a passthrough DOMRandomizer that returns input unchanged.
type NoopDOMRandomizer struct{}

// RandomizeDOM returns the input HTML unchanged.
func (n *NoopDOMRandomizer) RandomizeDOM(html string, seed int64) (string, map[string]any, error) {
	return html, map[string]any{"strategy": "noop"}, nil
}

// NoopCSSRandomizer is a passthrough CSSRandomizer that returns input unchanged.
type NoopCSSRandomizer struct{}

// RandomizeCSS returns the input HTML and CSS unchanged.
func (n *NoopCSSRandomizer) RandomizeCSS(html string, css string, seed int64) (string, string, map[string]any, error) {
	return html, css, map[string]any{"strategy": "noop"}, nil
}

// NoopAssetRandomizer is a passthrough AssetRandomizer that returns input unchanged.
type NoopAssetRandomizer struct{}

// RandomizeAssets returns the input files, HTML, and CSS unchanged.
func (n *NoopAssetRandomizer) RandomizeAssets(files map[string][]byte, html string, css string, seed int64) (map[string][]byte, string, string, map[string]any, error) {
	return files, html, css, map[string]any{"strategy": "noop"}, nil
}

// NoopDecoyInjector is a passthrough DecoyInjector that returns input unchanged.
type NoopDecoyInjector struct{}

// InjectDecoys returns the input HTML, CSS, and JS unchanged.
func (n *NoopDecoyInjector) InjectDecoys(html string, css string, js string, seed int64) (string, string, string, map[string]any, error) {
	return html, css, js, map[string]any{"strategy": "noop"}, nil
}

// NoopHeaderRandomizer is a passthrough HeaderRandomizer that returns empty headers.
type NoopHeaderRandomizer struct{}

// GenerateHeaderProfile returns an empty header profile with no middleware.
func (n *NoopHeaderRandomizer) GenerateHeaderProfile(seed int64) (map[string]string, string, map[string]any, error) {
	return map[string]string{}, "", map[string]any{"strategy": "noop"}, nil
}
