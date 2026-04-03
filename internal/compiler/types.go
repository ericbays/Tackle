// Package compiler implements the landing page compilation engine that transforms
// JSON page definitions into standalone Go binaries.
package compiler

// BuildInput contains the parameters for a compilation build.
type BuildInput struct {
	// Seed for deterministic randomization. Auto-generated if zero.
	Seed int64 `json:"seed,omitempty"`
	// Strategy override (e.g., "spa", "multifile", "hybrid"). Empty = random.
	Strategy string `json:"strategy,omitempty"`
	// CampaignID associates this build with a campaign.
	CampaignID string `json:"campaign_id,omitempty"`
	// Config holds campaign-specific build configuration.
	Config CampaignBuildConfig `json:"config"`
	// DisableRandomization skips all anti-fingerprinting randomization for debug/test builds.
	DisableRandomization bool `json:"disable_randomization,omitempty"`
}

// CampaignBuildConfig holds campaign-specific configuration baked into the compiled binary.
type CampaignBuildConfig struct {
	// FrameworkBaseURL is the localhost URL of the framework server (e.g., "http://127.0.0.1:8080").
	FrameworkBaseURL string `json:"framework_base_url"`
	// PostCaptureAction determines what happens after credential capture.
	// Values: "redirect", "display_page", "delay_redirect", "replay", "no_action"
	PostCaptureAction string `json:"post_capture_action"`
	// PostCaptureRedirectURL is used when PostCaptureAction is "redirect" or "delay_redirect".
	PostCaptureRedirectURL string `json:"post_capture_redirect_url,omitempty"`
	// PostCaptureDelayMs is the delay in milliseconds for "delay_redirect".
	PostCaptureDelayMs int `json:"post_capture_delay_ms,omitempty"`
	// PostCapturePageRoute is the page route to display for "display_page".
	PostCapturePageRoute string `json:"post_capture_page_route,omitempty"`
	// PostCaptureReplayURL is the URL to replay the form submission to for "replay".
	PostCaptureReplayURL string `json:"post_capture_replay_url,omitempty"`
	// TrackingTokenParam is the URL query parameter name for tracking tokens (default "t").
	TrackingTokenParam string `json:"tracking_token_param,omitempty"`
	// TargetOS is the target operating system for compilation (default: runtime GOOS).
	TargetOS string `json:"target_os,omitempty"`
	// TargetArch is the target architecture for compilation (default: runtime GOARCH).
	TargetArch string `json:"target_arch,omitempty"`
}

// BuildResult contains the output of a successful compilation.
type BuildResult struct {
	// BinaryPath is the absolute path to the compiled binary.
	BinaryPath string `json:"binary_path"`
	// BinaryHash is the SHA-256 hash of the compiled binary.
	BinaryHash string `json:"binary_hash"`
	// Manifest captures all build decisions for forensic review.
	Manifest BuildManifest `json:"manifest"`
	// Log is the complete build log.
	Log string `json:"log"`
}

// BuildManifest records all compilation decisions for forensic review.
type BuildManifest struct {
	// Seed used for randomization.
	Seed int64 `json:"seed"`
	// Strategy used for code generation.
	Strategy string `json:"strategy"`
	// GeneratedFiles lists all files produced during compilation.
	GeneratedFiles []string `json:"generated_files"`
	// PageCount is the number of pages in the definition.
	PageCount int `json:"page_count"`
	// ComponentCount is the total number of components across all pages.
	ComponentCount int `json:"component_count"`
	// FormFieldCount is the total number of form fields with capture tags.
	FormFieldCount int `json:"form_field_count"`
	// RandomizationEnabled indicates whether anti-fingerprinting was applied.
	RandomizationEnabled bool `json:"randomization_enabled"`
	// Randomization captures anti-fingerprinting decisions (populated by randomizers).
	Randomization RandomizationManifest `json:"randomization"`
}

// RandomizationManifest captures all anti-fingerprinting decisions.
type RandomizationManifest struct {
	DOM    map[string]any `json:"dom,omitempty"`
	CSS    map[string]any `json:"css,omitempty"`
	Assets map[string]any `json:"assets,omitempty"`
	Decoys map[string]any `json:"decoys,omitempty"`
	Headers map[string]any `json:"headers,omitempty"`
}

// PageAssets holds the generated HTML/CSS/JS for a single page.
type PageAssets struct {
	// Route is the URL path for this page.
	Route string
	// HTML is the complete HTML document.
	HTML string
	// Filename is the output filename (e.g., "index.html").
	Filename string
}

// GeneratedAssets holds all generated frontend assets.
type GeneratedAssets struct {
	// Pages maps route to generated page assets.
	Pages []PageAssets
	// GlobalCSS is the combined global stylesheet content.
	GlobalCSS string
	// GlobalJS is the combined global JavaScript content.
	GlobalJS string
}
