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

	"tackle/internal/compiler/gogen"
	"tackle/internal/compiler/htmlgen"
	"tackle/internal/compiler/randomizer"
	"tackle/internal/repositories"
	auditsvc "tackle/internal/services/audit"
)

// CompilationEngine transforms JSON page definitions into standalone Go binaries.
type CompilationEngine struct {
	repo             *repositories.LandingPageRepository
	auditSvc         *auditsvc.AuditService
	domRandomizer    randomizer.DOMRandomizer
	cssRandomizer    randomizer.CSSRandomizer
	assetRandomizer  randomizer.AssetRandomizer
	decoyInjector    randomizer.DecoyInjector
	headerRandomizer randomizer.HeaderRandomizer
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
		domRandomizer:    randomizer.NewLiveDOMRandomizer(),
		cssRandomizer:    randomizer.NewLiveCSSRandomizer(),
		assetRandomizer:  randomizer.NewLiveAssetRandomizer(),
		decoyInjector:    randomizer.NewLiveDecoyInjector(),
		headerRandomizer: randomizer.NewLiveHeaderRandomizer(),
		buildBaseDir:     config.BuildBaseDir,
		goBinary:         config.GoBinary,
		frameworkBaseURL: config.FrameworkBaseURL,
		logger:           config.Logger,
	}
}

// SetDOMRandomizer replaces the DOM randomizer (for Cline integration).
func (e *CompilationEngine) SetDOMRandomizer(r randomizer.DOMRandomizer) { e.domRandomizer = r }

// SetCSSRandomizer replaces the CSS randomizer (for Cline integration).
func (e *CompilationEngine) SetCSSRandomizer(r randomizer.CSSRandomizer) { e.cssRandomizer = r }

// SetAssetRandomizer replaces the asset randomizer (for Cline integration).
func (e *CompilationEngine) SetAssetRandomizer(r randomizer.AssetRandomizer) {
	e.assetRandomizer = r
}

// SetDecoyInjector replaces the decoy injector (for Cline integration).
func (e *CompilationEngine) SetDecoyInjector(r randomizer.DecoyInjector) { e.decoyInjector = r }

// SetHeaderRandomizer replaces the header randomizer (for Cline integration).
func (e *CompilationEngine) SetHeaderRandomizer(r randomizer.HeaderRandomizer) {
	e.headerRandomizer = r
}

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
	go e.runBuild(build.ID, project.DefinitionJSON, seed, strategy, buildToken, input)

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

func (e *CompilationEngine) runBuild(buildID string, def map[string]any, seed int64, strategy, buildToken string, input BuildInput) {
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

	// Step 1: Generate HTML/CSS/JS.
	logStep("Generating HTML/CSS/JS from page definition")

	frameworkURL := input.Config.FrameworkBaseURL
	if frameworkURL == "" {
		frameworkURL = e.frameworkBaseURL
	}

	tokenParam := input.Config.TrackingTokenParam
	if tokenParam == "" {
		tokenParam = "t"
	}

	pageConfig := htmlgen.PageConfig{
		CaptureEndpoint:        "/capture",
		TrackingEndpoint:       "/track",
		TrackingTokenParam:     tokenParam,
		PostCaptureAction:      input.Config.PostCaptureAction,
		PostCaptureRedirectURL: input.Config.PostCaptureRedirectURL,
		PostCaptureDelayMs:     input.Config.PostCaptureDelayMs,
		PostCapturePageRoute:   input.Config.PostCapturePageRoute,
		PostCaptureReplayURL:   input.Config.PostCaptureReplayURL,
	}

	pageOutputs, err := htmlgen.GeneratePageAssets(def, pageConfig)
	if err != nil {
		failBuild(fmt.Errorf("generate HTML: %w", err))
		return
	}
	logStep(fmt.Sprintf("Generated %d page(s)", len(pageOutputs)))

	// Step 2: Apply anti-fingerprinting randomization.
	manifest := BuildManifest{
		Seed:                 seed,
		Strategy:             strategy,
		PageCount:            len(pageOutputs),
		ComponentCount:       htmlgen.CountComponents(def),
		FormFieldCount:       htmlgen.CountCaptureFields(def),
		RandomizationEnabled: !input.DisableRandomization,
	}

	var headerMiddlewareSrc string

	if input.DisableRandomization {
		logStep("Anti-fingerprinting randomization DISABLED (debug build)")
	} else {
		logStep("Applying anti-fingerprinting randomization")

		// Apply DOM randomization to each page.
		for i, po := range pageOutputs {
			randomizedHTML, domManifest, err := e.domRandomizer.RandomizeDOM(po.HTML, seed+int64(i))
			if err != nil {
				failBuild(fmt.Errorf("DOM randomization: %w", err))
				return
			}
			pageOutputs[i].HTML = randomizedHTML
			if manifest.Randomization.DOM == nil {
				manifest.Randomization.DOM = domManifest
			}
		}

		// Apply CSS randomization.
		for i, po := range pageOutputs {
			randomizedHTML, randomizedCSS, cssManifest, err := e.cssRandomizer.RandomizeCSS(po.HTML, "", seed)
			if err != nil {
				failBuild(fmt.Errorf("CSS randomization: %w", err))
				return
			}
			pageOutputs[i].HTML = randomizedHTML
			_ = randomizedCSS // CSS already embedded in HTML
			if manifest.Randomization.CSS == nil {
				manifest.Randomization.CSS = cssManifest
			}
		}

		// Apply decoy injection.
		for i, po := range pageOutputs {
			randomizedHTML, _, _, decoyManifest, err := e.decoyInjector.InjectDecoys(po.HTML, "", "", seed+int64(i))
			if err != nil {
				failBuild(fmt.Errorf("decoy injection: %w", err))
				return
			}
			pageOutputs[i].HTML = randomizedHTML
			if manifest.Randomization.Decoys == nil {
				manifest.Randomization.Decoys = decoyManifest
			}
		}

		// Generate header profile.
		var headerManifest map[string]any
		_, headerMiddlewareSrc, headerManifest, err = e.headerRandomizer.GenerateHeaderProfile(seed)
		if err != nil {
			failBuild(fmt.Errorf("header randomization: %w", err))
			return
		}
		manifest.Randomization.Headers = headerManifest

		logStep("Anti-fingerprinting randomization complete")
	}

	// Step 3: Generate Go source code.
	logStep("Generating Go source code")

	goConfig := gogen.GoSourceConfig{
		ModuleName:             "landingpage",
		BuildToken:             buildToken,
		CampaignID:             input.CampaignID,
		FrameworkBaseURL:       frameworkURL,
		PostCaptureAction:      input.Config.PostCaptureAction,
		PostCaptureRedirectURL: input.Config.PostCaptureRedirectURL,
		PostCaptureDelayMs:     input.Config.PostCaptureDelayMs,
		PostCapturePageRoute:   input.Config.PostCapturePageRoute,
		PostCaptureReplayURL:   input.Config.PostCaptureReplayURL,
		HeaderMiddlewareSrc:    headerMiddlewareSrc,
		Pages:                  pageOutputs,
	}

	goSource, err := gogen.GenerateGoSource(goConfig)
	if err != nil {
		failBuild(fmt.Errorf("generate Go source: %w", err))
		return
	}
	logStep(fmt.Sprintf("Generated %d Go source file(s)", len(goSource.Files)))

	// Step 4: Write files to build directory.
	buildDir := filepath.Join(e.buildBaseDir, buildID)
	staticDir := filepath.Join(buildDir, "static")
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		failBuild(fmt.Errorf("create build dir: %w", err))
		return
	}
	logStep(fmt.Sprintf("Build directory: %s", buildDir))

	var generatedFiles []string

	// Write Go source files.
	for filename, content := range goSource.Files {
		path := filepath.Join(buildDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			failBuild(fmt.Errorf("write %s: %w", filename, err))
			return
		}
		generatedFiles = append(generatedFiles, filename)
	}

	// Write HTML files into static/.
	for _, po := range pageOutputs {
		path := filepath.Join(staticDir, po.Filename)
		if err := os.WriteFile(path, []byte(po.HTML), 0644); err != nil {
			failBuild(fmt.Errorf("write %s: %w", po.Filename, err))
			return
		}
		generatedFiles = append(generatedFiles, "static/"+po.Filename)
	}

	// Apply asset randomization (skip if disabled).
	if !input.DisableRandomization {
		files := make(map[string][]byte)
		for _, po := range pageOutputs {
			files["static/"+po.Filename] = []byte(po.HTML)
		}
		randomizedFiles, _, _, assetManifest, err := e.assetRandomizer.RandomizeAssets(files, "", "", seed)
		if err != nil {
			failBuild(fmt.Errorf("asset randomization: %w", err))
			return
		}
		manifest.Randomization.Assets = assetManifest

		// Write any new/renamed files from asset randomization.
		for filename, content := range randomizedFiles {
			path := filepath.Join(buildDir, filename)
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				failBuild(fmt.Errorf("create dir for %s: %w", filename, err))
				return
			}
			if err := os.WriteFile(path, content, 0644); err != nil {
				failBuild(fmt.Errorf("write randomized %s: %w", filename, err))
				return
			}
		}
	}

	manifest.GeneratedFiles = generatedFiles

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
