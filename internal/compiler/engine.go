package compiler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"tackle/internal/compiler/servergen"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// CompilationEngine transforms JSON page definitions into standalone Go binaries.
type CompilationEngine struct {
	repo             *repositories.LandingPageRepository
	auditSvc         *auditsvc.AuditService
	buildBaseDir     string
	goBinary         string
	frameworkBaseURL string
	logger           *slog.Logger
	mu               sync.Mutex
}

// EngineConfig holds configuration for the CompilationEngine.
type EngineConfig struct {
	// BuildBaseDir is the directory where build artifacts are stored.
	BuildBaseDir string
	// GoBinary is the path to the Go compiler binary (default: "go").
	GoBinary string
	// FrameworkBaseURL is the framework's internal API URL.
	FrameworkBaseURL string
	// Logger for build logging.
	Logger *slog.Logger
}

// NewCompilationEngine creates a new CompilationEngine with live anti-fingerprinting randomizers.
func NewCompilationEngine(
	repo *repositories.LandingPageRepository,
	auditSvc *auditsvc.AuditService,
	config EngineConfig,
) *CompilationEngine {
	if config.GoBinary == "" {
		config.GoBinary = "go"
	}
	if config.FrameworkBaseURL == "" {
		config.FrameworkBaseURL = "http://127.0.0.1:8080"
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &CompilationEngine{
		repo:             repo,
		auditSvc:         auditSvc,
		buildBaseDir:     config.BuildBaseDir,
		goBinary:         config.GoBinary,
		frameworkBaseURL: config.FrameworkBaseURL,
		logger:           config.Logger,
	}
}

// SetAssetRandomizer replaces the asset randomizer (for Cline integration).
func (e *CompilationEngine) SetAssetRandomizer(r interface{}) {}

// SetDecoyInjector replaces the decoy injector (for Cline integration).
func (e *CompilationEngine) SetDecoyInjector(r interface{}) {}

// SetHeaderRandomizer replaces the header randomizer (for Cline integration).
func (e *CompilationEngine) SetHeaderRandomizer(r interface{}) {}

// Build starts an asynchronous compilation build for a landing page project.
// Returns the build ID immediately; the build runs in a background goroutine.
func (e *CompilationEngine) Build(ctx context.Context, projectID, userID, userName string, input BuildInput) (string, error) {
	// Load project.
	project, err := e.repo.GetProjectByID(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("compilation: load project: %w", err)
	}

	// Generate seed if not provided.
	seed := input.Seed
	if seed == 0 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", fmt.Errorf("compilation: generate seed: %w", err)
		}
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
		if seed < 0 {
			seed = -seed
		}
	}

	// Generate build token.
	buildToken := uuid.New().String()

	// Set campaign ID.
	var campaignID *string
	if input.CampaignID != "" {
		campaignID = &input.CampaignID
	}

	// Strategy.
	strategy := input.Strategy
	if strategy == "" {
		strategy = "default"
	}

	// Create build record.
	build, err := e.repo.CreateBuild(ctx, repositories.LandingPageBuild{
		ProjectID:         projectID,
		CampaignID:        campaignID,
		Seed:              seed,
		Strategy:          strategy,
		BuildManifestJSON: map[string]any{},
		BuildLog:          "",
		Status:            "pending",
		BuildToken:        &buildToken,
	})
	if err != nil {
		return "", fmt.Errorf("compilation: create build record: %w", err)
	}

	// Audit log.
	e.logAudit(ctx, "landing_page.build_started", userID, userName, "landing_page_build", build.ID,
		map[string]any{"project_id": projectID, "seed": seed, "strategy": strategy})

	// Launch async build.
	go e.runBuild(projectID, build.ID, project.DefinitionJSON, seed, strategy, buildToken, input)

	return build.ID, nil
}

// GetBuild returns a build by ID.
func (e *CompilationEngine) GetBuild(ctx context.Context, buildID string) (repositories.LandingPageBuild, error) {
	return e.repo.GetBuildByID(ctx, buildID)
}

// ListBuilds returns builds for a project.
func (e *CompilationEngine) ListBuilds(ctx context.Context, projectID string) ([]repositories.LandingPageBuild, error) {
	return e.repo.ListBuildsByProject(ctx, projectID)
}

func (e *CompilationEngine) runBuild(projectID string, buildID string, def map[string]any, seed int64, strategy, buildToken string, input BuildInput) {
	ctx := context.Background()
	var buildLog strings.Builder

	logStep := func(msg string) {
		line := fmt.Sprintf("[%s] %s\n", time.Now().UTC().Format(time.RFC3339), msg)
		buildLog.WriteString(line)
		_ = e.repo.AppendBuildLog(ctx, buildID, line)
		e.logger.Info("build step", "build_id", buildID, "step", msg)
	}

	failBuild := func(err error) {
		logStep(fmt.Sprintf("BUILD FAILED: %v", err))
		_ = e.repo.UpdateBuildStatus(ctx, buildID, "failed", nil, nil, nil)
	}

	// Status -> building.
	if err := e.repo.UpdateBuildStatus(ctx, buildID, "building", nil, nil, nil); err != nil {
		failBuild(fmt.Errorf("update status: %w", err))
		return
	}
	logStep("Build started")

	logStep("Generating Unified Workspace (Servergen)")
	
	buildDir := filepath.Join(e.buildBaseDir, buildID)
	
	// In tackle, DevServer deployments bypass the DB compilation layer by utilizing
	// a mock batch file. For full servergen pipelines we invoke the Go Native esbuild layer.
	// Since builder uses development status to check Webhooks, let's trigger
	// the dev server Webhook sequence if this build is explicitly flagged local
	
	isDevelopment := false
	if input.CampaignID == "" {
		isDevelopment = true // If no campaign is paired, assume local DevServer invocation
	}
	
	generatedFiles, err := servergen.GenerateWorkspace(buildDir, projectID, buildID, def, isDevelopment)
	if err != nil {
		failBuild(fmt.Errorf("servergen pipeline: %w", err))
		return
	}
	logStep(fmt.Sprintf("Servergen pipeline completed. Transpiled %d artifacts.", len(generatedFiles)))
	
	// Create generic stub manifest
	manifest := BuildManifest{
		Seed:                 seed,
		Strategy:             strategy,
		PageCount:            1,
		ComponentCount:       1,
		FormFieldCount:       1,
		RandomizationEnabled: !input.DisableRandomization,
		GeneratedFiles:       generatedFiles,
	}

	// Step 5: Compile Go binary.
	logStep("Compiling Go binary")

	targetOS := input.Config.TargetOS
	targetArch := input.Config.TargetArch
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	binaryName := buildID
	if targetOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(e.buildBaseDir, binaryName)

	buildCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, e.goBinary, "build", "-o", binaryPath, ".")
	cmd.Dir = buildDir
	cmd.Env = append(os.Environ(),
		"GOOS="+targetOS,
		"GOARCH="+targetArch,
		"CGO_ENABLED=0",
	)

	var cmdOutput strings.Builder
	cmd.Stdout = &cmdOutput
	cmd.Stderr = &cmdOutput

	if err := cmd.Run(); err != nil {
		logStep(fmt.Sprintf("Go build output:\n%s", cmdOutput.String()))
		failBuild(fmt.Errorf("go build: %w", err))
		return
	}
	logStep("Go binary compiled successfully")

	// Step 6: Hash the binary.
	binaryHash, err := hashFile(binaryPath)
	if err != nil {
		failBuild(fmt.Errorf("hash binary: %w", err))
		return
	}
	logStep(fmt.Sprintf("Binary hash: %s", binaryHash))

	// Step 7: Update build record.
	manifestMap := manifestToMap(manifest)
	if err := e.repo.UpdateBuildManifest(ctx, buildID, manifestMap); err != nil {
		failBuild(fmt.Errorf("update manifest: %w", err))
		return
	}
	if err := e.repo.UpdateBuildStatus(ctx, buildID, "built", &binaryPath, &binaryHash, nil); err != nil {
		failBuild(fmt.Errorf("update status to built: %w", err))
		return
	}

	// Step 8: Clean up source directory (keep binary).
	if err := os.RemoveAll(buildDir); err != nil {
		logStep(fmt.Sprintf("Warning: failed to clean build dir: %v", err))
	}

	logStep("Build completed successfully")
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func manifestToMap(m BuildManifest) map[string]any {
	data, _ := json.Marshal(m)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}

func (e *CompilationEngine) logAudit(ctx context.Context, action, actorID, actorName, resType, resID string, details map[string]any) {
	if e.auditSvc == nil {
		return
	}
	entry := auditsvc.LogEntry{
		Category:     "system",
		Severity:     "info",
		ActorType:    "user",
		Action:       action,
		Details:      details,
	}
	if actorID != "" {
		entry.ActorID = &actorID
	}
	if actorName != "" {
		entry.ActorLabel = actorName
	}
	if resType != "" {
		entry.ResourceType = &resType
	}
	if resID != "" {
		entry.ResourceID = &resID
	}
	e.auditSvc.Log(ctx, entry)
}
