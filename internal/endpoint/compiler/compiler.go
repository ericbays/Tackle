// Package compiler implements per-deployment unique binary compilation for phishing endpoint proxies.
package compiler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CompileInput holds the parameters for a binary compilation.
type CompileInput struct {
	CampaignID      string
	EndpointID      string
	FrameworkHost   string // Base URL of the framework server
	LandingPagePort int    // Port where the landing page app is hosted on the framework
	ControlPort     int    // Port for the control channel
	AuthToken       string // Pre-shared auth token for framework communication
	TargetArch      string // "amd64" or "arm64"
}

// CompileResult holds the output of a binary compilation.
type CompileResult struct {
	BinaryPath string // Absolute path to the compiled binary
	BinaryHash string // SHA-256 hex digest of the compiled binary
	Arch       string // Target architecture
	BuildLog   string // Compilation log output
}

// Compiler produces unique proxy binaries for each deployment.
type Compiler struct {
	goBinary    string // Path to the Go compiler binary
	buildDir    string // Base directory for build artifacts
	templateDir string // Path to the source template directory
}

// NewCompiler creates a new Compiler.
func NewCompiler(goBinary, buildDir string) *Compiler {
	return &Compiler{
		goBinary: goBinary,
		buildDir: buildDir,
	}
}

// SetTemplateDir sets a custom template directory. If not set, the compiler
// resolves the template relative to the source file location.
func (c *Compiler) SetTemplateDir(dir string) {
	c.templateDir = dir
}

// Compile produces a unique statically-linked binary for the given deployment.
// Each call produces a binary with a unique SHA-256 hash.
func (c *Compiler) Compile(ctx context.Context, input CompileInput) (CompileResult, error) {
	startTime := time.Now()
	result := CompileResult{Arch: input.TargetArch}

	if err := c.validateInput(input); err != nil {
		return result, fmt.Errorf("validate input: %w", err)
	}

	// Create temporary build directory.
	buildDir, err := c.createBuildDir(input.EndpointID)
	if err != nil {
		return result, fmt.Errorf("create build dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	// Copy source template to build directory.
	if err := c.copyTemplate(buildDir); err != nil {
		return result, fmt.Errorf("copy template: %w", err)
	}

	// Generate unique entropy for this build.
	nonce, err := generateNonce()
	if err != nil {
		return result, fmt.Errorf("generate nonce: %w", err)
	}
	deployTimestamp := fmt.Sprintf("%d", time.Now().UnixNano())

	// Compile the binary with ldflags injection.
	binaryPath, buildOutput, err := c.compileBinary(ctx, buildDir, input, nonce, deployTimestamp)
	if err != nil {
		return result, fmt.Errorf("compile binary: %w", err)
	}

	// Compute SHA-256 hash.
	binaryHash, err := computeFileHash(binaryPath)
	if err != nil {
		return result, fmt.Errorf("compute binary hash: %w", err)
	}

	// Move binary to stable output location.
	finalPath, err := c.moveBinaryToOutput(binaryPath, input.EndpointID, input.TargetArch)
	if err != nil {
		return result, fmt.Errorf("move binary to output: %w", err)
	}

	result.BinaryPath = finalPath
	result.BinaryHash = binaryHash
	result.BuildLog = c.buildSummary(input, nonce, deployTimestamp, finalPath, binaryHash, buildOutput, startTime)

	return result, nil
}

// Cleanup removes build artifacts for a specific endpoint.
func (c *Compiler) Cleanup(endpointID string) error {
	outputDir := filepath.Join(c.buildDir, "output", endpointID)
	return os.RemoveAll(outputDir)
}

// validateInput performs basic validation on the compile input.
func (c *Compiler) validateInput(input CompileInput) error {
	if input.CampaignID == "" {
		return fmt.Errorf("campaign ID is required")
	}
	if input.EndpointID == "" {
		return fmt.Errorf("endpoint ID is required")
	}
	if input.FrameworkHost == "" {
		return fmt.Errorf("framework host is required")
	}
	switch input.TargetArch {
	case "amd64", "arm64":
		// valid
	default:
		return fmt.Errorf("unsupported architecture: %s", input.TargetArch)
	}
	return nil
}

// createBuildDir creates a fresh temporary directory for the build.
func (c *Compiler) createBuildDir(endpointID string) (string, error) {
	if err := os.MkdirAll(c.buildDir, 0755); err != nil {
		return "", fmt.Errorf("create build dir: %w", err)
	}
	return os.MkdirTemp(c.buildDir, "build-"+endpointID[:min(8, len(endpointID))]+"-")
}

// resolveTemplateDir returns the absolute path to the source template directory.
func (c *Compiler) resolveTemplateDir() (string, error) {
	if c.templateDir != "" {
		if _, err := os.Stat(c.templateDir); err != nil {
			return "", fmt.Errorf("template directory not found: %w", err)
		}
		return c.templateDir, nil
	}

	// Resolve relative to this source file's location.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine source file location")
	}
	templateDir := filepath.Join(filepath.Dir(thisFile), "template")
	if _, err := os.Stat(templateDir); err != nil {
		return "", fmt.Errorf("template directory not found at %s: %w", templateDir, err)
	}
	return templateDir, nil
}

// copyTemplate copies the source template files to the build directory.
func (c *Compiler) copyTemplate(destDir string) error {
	templateDir, err := c.resolveTemplateDir()
	if err != nil {
		return err
	}

	return filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == templateDir {
			return nil
		}

		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		return copyFile(path, destPath)
	})
}

// compileBinary runs the Go compiler to produce the binary.
func (c *Compiler) compileBinary(ctx context.Context, buildDir string, input CompileInput, nonce, deployTimestamp string) (string, string, error) {
	// Validate Go binary exists.
	if err := c.validateGoBinary(ctx); err != nil {
		return "", "", fmt.Errorf("go binary: %w", err)
	}

	env := append(os.Environ(),
		"GOOS=linux",
		"GOARCH="+input.TargetArch,
		"CGO_ENABLED=0",
	)

	binaryPath := filepath.Join(buildDir, "proxy")

	// Build ldflags to inject unique build-time variables.
	ldflags := fmt.Sprintf(
		"-X main.campaignID=%s -X main.endpointID=%s -X main.deployTimestamp=%s -X main.buildNonce=%s -X main.frameworkHost=%s -X main.landingPagePort=%d -X main.controlPort=%d -X main.authToken=%s -s -w",
		input.CampaignID,
		input.EndpointID,
		deployTimestamp,
		nonce,
		input.FrameworkHost,
		input.LandingPagePort,
		input.ControlPort,
		input.AuthToken,
	)

	cmd := exec.CommandContext(ctx, c.goBinary, "build",
		"-trimpath",
		"-ldflags", ldflags,
		"-o", binaryPath,
		".",
	)
	cmd.Dir = buildDir
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(output), fmt.Errorf("go build: %w\n%s", err, string(output))
	}

	return binaryPath, string(output), nil
}

// validateGoBinary checks that the Go binary exists and is functional.
func (c *Compiler) validateGoBinary(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.goBinary, "version")
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("go binary not found or not executable: %w", err)
	}
	return nil
}

// getGoVersion returns the Go compiler version string.
func (c *Compiler) getGoVersion(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, c.goBinary, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// moveBinaryToOutput moves the compiled binary to a stable output location.
func (c *Compiler) moveBinaryToOutput(srcPath, endpointID, arch string) (string, error) {
	outputDir := filepath.Join(c.buildDir, "output", endpointID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	baseName := "proxy"
	if arch == "arm64" {
		baseName += "-arm64"
	}
	finalPath := filepath.Join(outputDir, baseName)

	if err := copyFile(srcPath, finalPath); err != nil {
		return "", fmt.Errorf("copy binary to output: %w", err)
	}
	return finalPath, nil
}

// buildSummary generates a human-readable summary of the build.
func (c *Compiler) buildSummary(input CompileInput, nonce, deployTimestamp, binaryPath, binaryHash, buildOutput string, startTime time.Time) string {
	var log strings.Builder

	log.WriteString("--- Build Summary ---\n")
	log.WriteString(fmt.Sprintf("Build timestamp: %s\n", startTime.UTC().Format(time.RFC3339)))
	log.WriteString(fmt.Sprintf("Architecture: %s\n", input.TargetArch))
	log.WriteString(fmt.Sprintf("Go version: %s\n", c.getGoVersion(context.Background())))
	log.WriteString(fmt.Sprintf("Campaign ID (truncated): %s...\n", truncate(input.CampaignID, 8)))
	log.WriteString(fmt.Sprintf("Endpoint ID (truncated): %s...\n", truncate(input.EndpointID, 8)))
	log.WriteString(fmt.Sprintf("Build nonce: %s...\n", truncate(nonce, 16)))
	log.WriteString(fmt.Sprintf("Deploy timestamp: %s\n", deployTimestamp))
	log.WriteString(fmt.Sprintf("Framework host: %s\n", input.FrameworkHost))
	log.WriteString(fmt.Sprintf("Landing page port: %d\n", input.LandingPagePort))
	log.WriteString(fmt.Sprintf("Control port: %d\n", input.ControlPort))
	log.WriteString(fmt.Sprintf("Auth token: (masked)\n"))
	log.WriteString(fmt.Sprintf("Binary path: %s\n", binaryPath))

	if info, err := os.Stat(binaryPath); err == nil {
		log.WriteString(fmt.Sprintf("Binary size: %d bytes\n", info.Size()))
	}

	log.WriteString(fmt.Sprintf("Binary hash: %s\n", binaryHash))
	log.WriteString(fmt.Sprintf("Compilation duration: %s\n", time.Since(startTime).Round(time.Millisecond)))
	log.WriteString("Success: binary compiled with unique hash\n")

	if buildOutput != "" {
		log.WriteString(fmt.Sprintf("Compiler output: %s\n", buildOutput))
	}

	return log.String()
}

// generateNonce generates a cryptographically secure random nonce for the build.
func generateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read nonce bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// computeFileHash calculates the SHA-256 hash of a file.
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// truncate returns at most maxLen characters of s, or the full string if shorter.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

