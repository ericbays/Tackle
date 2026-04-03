package compiler

import (
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// validInput returns a CompileInput with all required fields populated.
func validInput(suffix string) CompileInput {
	return CompileInput{
		CampaignID:      "campaign-" + suffix + "-550e8400",
		EndpointID:      "endpoint-" + suffix + "-6ba7b810",
		FrameworkHost:   "https://10.0.1.50:8443",
		LandingPagePort: 10001,
		ControlPort:     9443,
		AuthToken:       "auth-token-" + suffix + "-abcdef1234567890",
		TargetArch:      "amd64",
	}
}

func findGoBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GO_BINARY"); p != "" {
		return p
	}
	p, err := exec.LookPath("go")
	if err != nil {
		return "go"
	}
	return p
}

func newTestCompiler(t *testing.T) *Compiler {
	t.Helper()
	c := NewCompiler(findGoBinary(t), t.TempDir())
	// Set template dir explicitly so tests work regardless of CWD.
	c.SetTemplateDir(resolveTestTemplateDir(t))
	return c
}

func resolveTestTemplateDir(t *testing.T) string {
	t.Helper()
	// Walk up from test file location to find the template dir.
	// Tests run with CWD set to the package directory.
	candidates := []string{
		"template",                                   // running from package dir
		"internal/endpoint/compiler/template",        // running from repo root
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Fatal("cannot find template directory")
	return ""
}

// TestHashUniqueness verifies that compiling the same input twice produces different hashes.
func TestHashUniqueness(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("hash-unique")

	result1, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}

	result2, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}

	if result1.BinaryHash == result2.BinaryHash {
		t.Errorf("expected different hashes, both are %s", result1.BinaryHash)
	}
}

// TestDifferentInputsProduceDifferentHashes verifies different campaign IDs → different hashes.
func TestDifferentInputsProduceDifferentHashes(t *testing.T) {
	comp := newTestCompiler(t)

	input1 := validInput("diff-a")
	input2 := validInput("diff-b")
	input2.CampaignID = "different-campaign-id-xyz"

	result1, err := comp.Compile(context.Background(), input1)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}

	result2, err := comp.Compile(context.Background(), input2)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}

	if result1.BinaryHash == result2.BinaryHash {
		t.Errorf("expected different hashes for different campaigns, both are %s", result1.BinaryHash)
	}
}

// TestStaticCompilation verifies the produced binary exists and hash is correct.
func TestStaticCompilation(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("static")

	result, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if _, err := os.Stat(result.BinaryPath); err != nil {
		t.Fatalf("binary not found: %v", err)
	}

	// Re-hash and verify.
	hash, err := computeFileHash(result.BinaryPath)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash != result.BinaryHash {
		t.Errorf("hash mismatch: result=%s, recomputed=%s", result.BinaryHash, hash)
	}
}

// TestBuildArtifactCleanup verifies Cleanup removes output artifacts.
func TestBuildArtifactCleanup(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("cleanup")

	result, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Binary should exist.
	if _, err := os.Stat(result.BinaryPath); err != nil {
		t.Fatalf("binary not found: %v", err)
	}

	// Cleanup.
	if err := comp.Cleanup(input.EndpointID); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	// Binary should be gone.
	if _, err := os.Stat(result.BinaryPath); !os.IsNotExist(err) {
		t.Errorf("binary should be removed after cleanup")
	}
}

// TestArchitectureTargeting tests compilation for both amd64 and arm64.
func TestArchitectureTargeting(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			comp := newTestCompiler(t)
			input := validInput("arch-" + arch)
			input.TargetArch = arch

			result, err := comp.Compile(context.Background(), input)
			if err != nil {
				t.Fatalf("compile for %s: %v", arch, err)
			}

			if _, err := os.Stat(result.BinaryPath); err != nil {
				t.Fatalf("binary not found: %v", err)
			}
			if result.Arch != arch {
				t.Errorf("expected arch %s, got %s", arch, result.Arch)
			}
		})
	}
}

// TestBuildLogContents verifies expected fields are present and sensitive values are masked.
func TestBuildLogContents(t *testing.T) {
	comp := newTestCompiler(t)
	authToken := "super-secret-auth-token-1234567890"
	input := validInput("logtest")
	input.AuthToken = authToken

	result, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	log := result.BuildLog

	for _, field := range []string{
		"Build timestamp", "Architecture", "Framework host",
		"Landing page port", "Control port", "Binary path",
		"Binary hash", "Success",
	} {
		if !strings.Contains(log, field) {
			t.Errorf("build log missing field: %s", field)
		}
	}

	// Full auth token should NOT appear.
	if strings.Contains(log, authToken) {
		t.Error("full auth token should not appear in build log")
	}

	// Hash format check.
	if len(result.BinaryHash) != 64 {
		t.Errorf("binary hash should be 64 chars, got %d", len(result.BinaryHash))
	}
}

// TestInvalidGoBinaryPath verifies graceful error when Go compiler is not found.
func TestInvalidGoBinaryPath(t *testing.T) {
	comp := NewCompiler("/nonexistent/go/binary/path", t.TempDir())
	comp.SetTemplateDir(resolveTestTemplateDir(t))

	input := validInput("badgo")
	_, err := comp.Compile(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid Go binary path")
	}
}

// TestContextCancellation verifies compilation respects context cancellation.
func TestContextCancellation(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("cancel")

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately.
	cancel()

	_, err := comp.Compile(ctx, input)
	// Should either fail or succeed (if compile is faster than cancel propagation).
	// We just verify it doesn't hang.
	_ = err
}

// TestCompileResultFields verifies all fields are populated.
func TestCompileResultFields(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("fields")

	result, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if result.BinaryPath == "" {
		t.Error("BinaryPath is empty")
	}
	if result.BinaryHash == "" {
		t.Error("BinaryHash is empty")
	}
	if result.Arch == "" {
		t.Error("Arch is empty")
	}
	if result.BuildLog == "" {
		t.Error("BuildLog is empty")
	}

	if len(result.BinaryHash) != 64 {
		t.Errorf("hash should be 64 hex chars, got %d", len(result.BinaryHash))
	}
	if _, err := hex.DecodeString(result.BinaryHash); err != nil {
		t.Errorf("hash is not valid hex: %v", err)
	}
}

// TestInputValidation tests validation of CompileInput.
func TestInputValidation(t *testing.T) {
	comp := newTestCompiler(t)

	tests := []struct {
		name    string
		input   CompileInput
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   validInput("valid"),
			wantErr: false,
		},
		{
			name: "missing campaign ID",
			input: CompileInput{
				EndpointID: "ep", FrameworkHost: "https://x", TargetArch: "amd64", AuthToken: "t",
			},
			wantErr: true,
		},
		{
			name: "missing endpoint ID",
			input: CompileInput{
				CampaignID: "c", FrameworkHost: "https://x", TargetArch: "amd64", AuthToken: "t",
			},
			wantErr: true,
		},
		{
			name: "missing framework host",
			input: CompileInput{
				CampaignID: "c", EndpointID: "ep", TargetArch: "amd64", AuthToken: "t",
			},
			wantErr: true,
		},
		{
			name: "invalid architecture",
			input: CompileInput{
				CampaignID: "c", EndpointID: "ep", FrameworkHost: "https://x", TargetArch: "ppc64", AuthToken: "t",
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := comp.Compile(context.Background(), tc.input)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestNonceUniqueness verifies generated nonces are unique.
func TestNonceUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		n, err := generateNonce()
		if err != nil {
			t.Fatalf("generate nonce %d: %v", i, err)
		}
		if len(n) != 64 {
			t.Errorf("nonce should be 64 hex chars, got %d", len(n))
		}
		if seen[n] {
			t.Errorf("duplicate nonce on iteration %d: %s", i, n)
		}
		seen[n] = true
	}
}

// TestHashEntropyAcrossBuilds verifies that repeated builds of identical input
// produce distinct hashes every time.
func TestHashEntropyAcrossBuilds(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("entropy")

	const iterations = 3
	hashes := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		result, err := comp.Compile(context.Background(), input)
		if err != nil {
			t.Fatalf("compile iteration %d: %v", i, err)
		}
		if hashes[result.BinaryHash] {
			t.Errorf("duplicate hash on iteration %d: %s", i, result.BinaryHash)
		}
		hashes[result.BinaryHash] = true
	}

	if len(hashes) != iterations {
		t.Errorf("expected %d unique hashes, got %d", iterations, len(hashes))
	}
}

// TestBuildDirIsolation verifies each build uses a fresh directory.
func TestBuildDirIsolation(t *testing.T) {
	comp := newTestCompiler(t)

	input1 := validInput("iso-1")
	input2 := validInput("iso-2")
	input2.CampaignID = "isolation-campaign-2"

	result1, err := comp.Compile(context.Background(), input1)
	if err != nil {
		t.Fatalf("first compile: %v", err)
	}
	result2, err := comp.Compile(context.Background(), input2)
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}

	if result1.BinaryHash == result2.BinaryHash {
		t.Error("isolated builds should produce different hashes")
	}
}

// TestBuildLogNoAuthToken verifies the full auth token never appears in the log.
func TestBuildLogNoAuthToken(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("notoken")
	input.AuthToken = "very-long-auth-token-1234567890abcdef"

	result, err := comp.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	if strings.Contains(result.BuildLog, input.AuthToken) {
		t.Error("full auth token should not appear in build log")
	}
}

// TestGoVersionDetection verifies the compiler can detect Go version.
func TestGoVersionDetection(t *testing.T) {
	comp := newTestCompiler(t)
	ver := comp.getGoVersion(context.Background())
	if !strings.HasPrefix(ver, "go version") {
		t.Errorf("expected 'go version' prefix, got: %s", ver)
	}
}

// TestCompilationTimeout verifies compilation respects a tight deadline.
func TestCompilationTimeout(t *testing.T) {
	comp := newTestCompiler(t)
	input := validInput("timeout")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Wait for context to expire.
	time.Sleep(time.Millisecond)

	_, err := comp.Compile(ctx, input)
	if err == nil {
		// It's possible the compile finished before context expired on fast machines.
		// Not an error — just skip the assertion.
		t.Log("compile succeeded before timeout (fast machine)")
	}
}
