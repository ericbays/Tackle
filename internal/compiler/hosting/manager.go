// Package hosting manages the lifecycle of compiled landing page applications.
package hosting

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"tackle/internal/repositories"
)

// AppManager manages running landing page application instances.
type AppManager struct {
	repo           *repositories.LandingPageRepository
	apps           map[string]*RunningApp
	mu             sync.RWMutex
	portRangeMin   int
	portRangeMax   int
	maxConcurrent  int
	healthInterval time.Duration
	maxRetries     int
	logger         *slog.Logger
	stopCh         chan struct{}
}

// RunningApp tracks a running landing page application process.
type RunningApp struct {
	BuildID    string
	Port       int
	Cmd        *exec.Cmd
	RetryCount int
	Healthy    bool
	FailCount  int // consecutive health check failures
	cancel     context.CancelFunc
}

// ManagerConfig holds configuration for AppManager.
type ManagerConfig struct {
	PortRangeMin   int           // Default: 10000
	PortRangeMax   int           // Default: 60000
	MaxConcurrent  int           // Default: 20
	HealthInterval time.Duration // Default: 30s
	MaxRetries     int           // Default: 3
	Logger         *slog.Logger
}

// NewAppManager creates a new AppManager.
func NewAppManager(repo *repositories.LandingPageRepository, config ManagerConfig) *AppManager {
	if config.PortRangeMin == 0 {
		config.PortRangeMin = 10000
	}
	if config.PortRangeMax == 0 {
		config.PortRangeMax = 60000
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 20
	}
	if config.HealthInterval == 0 {
		config.HealthInterval = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &AppManager{
		repo:           repo,
		apps:           make(map[string]*RunningApp),
		portRangeMin:   config.PortRangeMin,
		portRangeMax:   config.PortRangeMax,
		maxConcurrent:  config.MaxConcurrent,
		healthInterval: config.HealthInterval,
		maxRetries:     config.MaxRetries,
		logger:         config.Logger,
		stopCh:         make(chan struct{}),
	}
}

// CleanupStaleBuilds resets any builds stuck in 'running' or 'starting' status
// from a previous server process. Call this once after construction.
func (m *AppManager) CleanupStaleBuilds(ctx context.Context) {
	count, err := m.repo.ResetStaleBuilds(ctx)
	if err != nil {
		m.logger.Error("failed to reset stale builds", "error", err)
		return
	}
	if count > 0 {
		m.logger.Info("reset stale builds from previous server process", "count", count)
	}
}

// StartApp starts a compiled landing page application.
func (m *AppManager) StartApp(ctx context.Context, buildID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already running.
	if _, exists := m.apps[buildID]; exists {
		return fmt.Errorf("hosting: app %s is already running", buildID)
	}

	// Check concurrent limit.
	if len(m.apps) >= m.maxConcurrent {
		return fmt.Errorf("hosting: concurrent app limit reached (%d)", m.maxConcurrent)
	}

	// Get build record.
	build, err := m.repo.GetBuildByID(ctx, buildID)
	if err != nil {
		return fmt.Errorf("hosting: get build: %w", err)
	}
	// If DB says running/starting but it's not in our in-memory map, the server
	// must have restarted. Reset to stopped so we can start fresh.
	if build.Status == "running" || build.Status == "starting" {
		m.logger.Warn("stale build status detected, resetting to stopped",
			"build_id", buildID, "stale_status", build.Status)
		if err := m.repo.UpdateBuildStatus(ctx, buildID, "stopped", nil, nil, nil); err != nil {
			return fmt.Errorf("hosting: reset stale status: %w", err)
		}
		build.Status = "stopped"
	}
	if build.Status != "built" && build.Status != "stopped" {
		return fmt.Errorf("hosting: cannot start build in status %q", build.Status)
	}
	if build.BinaryPath == nil || *build.BinaryPath == "" {
		return fmt.Errorf("hosting: no binary path for build %s", buildID)
	}

	// Use the project's assigned port if available, otherwise select a new one and persist it.
	port, err := m.resolvePort(ctx, build.ProjectID)
	if err != nil {
		return fmt.Errorf("hosting: resolve port: %w", err)
	}

	// Update status to starting.
	if err := m.repo.UpdateBuildStatus(ctx, buildID, "starting", nil, nil, &port); err != nil {
		return fmt.Errorf("hosting: update status: %w", err)
	}

	// Start process.
	appCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(appCtx, *build.BinaryPath, "--port", fmt.Sprintf("%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		_ = m.repo.UpdateBuildStatus(ctx, buildID, "failed", nil, nil, nil)
		return fmt.Errorf("hosting: start process: %w", err)
	}

	app := &RunningApp{
		BuildID: buildID,
		Port:    port,
		Cmd:     cmd,
		Healthy: false,
		cancel:  cancel,
	}
	m.apps[buildID] = app

	// Wait for health check in background.
	go m.waitForHealthy(buildID, port)

	// Monitor process exit in background.
	go m.monitorProcess(buildID, app)

	m.logger.Info("app started", "build_id", buildID, "port", port, "pid", cmd.Process.Pid)
	return nil
}

func (m *AppManager) waitForHealthy(buildID string, port int) {
	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)

		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				m.mu.Lock()
				if app, ok := m.apps[buildID]; ok {
					app.Healthy = true
				}
				m.mu.Unlock()
				_ = m.repo.UpdateBuildStatus(ctx, buildID, "running", nil, nil, nil)
				m.logger.Info("app healthy", "build_id", buildID, "port", port)
				return
			}
		}
	}

	m.logger.Error("app failed health check startup", "build_id", buildID)
	_ = m.StopApp(ctx, buildID)
	_ = m.repo.UpdateBuildStatus(ctx, buildID, "failed", nil, nil, nil)
}

func (m *AppManager) monitorProcess(buildID string, app *RunningApp) {
	if app.Cmd.Process == nil {
		return
	}
	_ = app.Cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.apps[buildID]; exists {
		delete(m.apps, buildID)
		m.logger.Warn("app process exited unexpectedly", "build_id", buildID)
	}
}

// StopApp stops a running landing page application.
func (m *AppManager) StopApp(ctx context.Context, buildID string) error {
	m.mu.Lock()
	app, exists := m.apps[buildID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("hosting: app %s is not running", buildID)
	}
	delete(m.apps, buildID)
	m.mu.Unlock()

	_ = m.repo.UpdateBuildStatus(ctx, buildID, "stopping", nil, nil, nil)

	// Terminate the process.
	if app.Cmd.Process != nil {
		app.cancel()
		// Give it a moment to exit gracefully.
		done := make(chan struct{})
		go func() {
			_ = app.Cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Exited gracefully.
		case <-time.After(5 * time.Second):
			// Force kill.
			_ = app.Cmd.Process.Kill()
		}
	}

	_ = m.repo.UpdateBuildStatus(ctx, buildID, "stopped", nil, nil, nil)
	m.logger.Info("app stopped", "build_id", buildID)
	return nil
}

// CleanupApp stops a running app (if any) and deletes the binary from disk.
func (m *AppManager) CleanupApp(ctx context.Context, buildID string) error {
	// Stop if running.
	m.mu.RLock()
	_, running := m.apps[buildID]
	m.mu.RUnlock()
	if running {
		if err := m.StopApp(ctx, buildID); err != nil {
			m.logger.Warn("cleanup: stop failed", "build_id", buildID, "error", err)
		}
	}

	// Get build to find binary path.
	build, err := m.repo.GetBuildByID(ctx, buildID)
	if err != nil {
		return fmt.Errorf("hosting: cleanup: get build: %w", err)
	}

	// Delete binary.
	if build.BinaryPath != nil && *build.BinaryPath != "" {
		if err := os.Remove(*build.BinaryPath); err != nil && !os.IsNotExist(err) {
			m.logger.Warn("cleanup: delete binary failed", "path", *build.BinaryPath, "error", err)
		}
	}

	_ = m.repo.UpdateBuildStatus(ctx, buildID, "cleaned_up", nil, nil, nil)
	m.logger.Info("app cleaned up", "build_id", buildID)
	return nil
}

// GetHealth returns the health status for a running app.
func (m *AppManager) GetHealth(buildID string) (bool, int, error) {
	m.mu.RLock()
	app, exists := m.apps[buildID]
	m.mu.RUnlock()

	if !exists {
		return false, 0, fmt.Errorf("hosting: app %s is not running", buildID)
	}
	return app.Healthy, app.Port, nil
}

// RunningCount returns the number of currently running apps.
func (m *AppManager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.apps)
}

// StartHealthMonitor starts a background goroutine that periodically checks app health.
func (m *AppManager) StartHealthMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.healthInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.checkAllHealth(ctx)
			}
		}
	}()
}

func (m *AppManager) checkAllHealth(ctx context.Context) {
	m.mu.RLock()
	buildIDs := make([]string, 0, len(m.apps))
	ports := make(map[string]int)
	for id, app := range m.apps {
		buildIDs = append(buildIDs, id)
		ports[id] = app.Port
	}
	m.mu.RUnlock()

	client := &http.Client{Timeout: 5 * time.Second}

	for _, buildID := range buildIDs {
		port := ports[buildID]
		healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)

		start := time.Now()
		resp, err := client.Get(healthURL)
		responseTimeMs := int(time.Since(start).Milliseconds())

		status := "healthy"
		if err != nil || resp.StatusCode != http.StatusOK {
			status = "unhealthy"
		}
		if resp != nil {
			resp.Body.Close()
		}

		// Record health check.
		_, _ = m.repo.CreateHealthCheck(ctx, repositories.LandingPageHealthCheck{
			BuildID:        buildID,
			Status:         status,
			ResponseTimeMs: responseTimeMs,
		})

		m.mu.Lock()
		app, exists := m.apps[buildID]
		if exists {
			if status == "unhealthy" {
				app.FailCount++
				app.Healthy = false
				if app.FailCount >= 3 {
					m.logger.Warn("app unhealthy, attempting restart",
						"build_id", buildID, "fail_count", app.FailCount,
						"retry_count", app.RetryCount)

					if app.RetryCount < m.maxRetries {
						app.RetryCount++
						app.FailCount = 0
						// Restart: stop and start again.
						go func(bid string) {
							_ = m.StopApp(ctx, bid)
							time.Sleep(2 * time.Second)
							if err := m.StartApp(ctx, bid); err != nil {
								m.logger.Error("restart failed", "build_id", bid, "error", err)
								_ = m.repo.UpdateBuildStatus(ctx, bid, "failed", nil, nil, nil)
							}
						}(buildID)
					} else {
						m.logger.Error("app exceeded max retries", "build_id", buildID)
						go func(bid string) {
							_ = m.StopApp(ctx, bid)
							_ = m.repo.UpdateBuildStatus(ctx, bid, "failed", nil, nil, nil)
						}(buildID)
					}
				}
			} else {
				app.FailCount = 0
				app.Healthy = true
			}
		}
		m.mu.Unlock()
	}
}

// StopAll stops all running applications.
func (m *AppManager) StopAll(ctx context.Context) {
	m.mu.RLock()
	buildIDs := make([]string, 0, len(m.apps))
	for id := range m.apps {
		buildIDs = append(buildIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range buildIDs {
		if err := m.StopApp(ctx, id); err != nil {
			m.logger.Warn("stop all: failed to stop app", "build_id", id, "error", err)
		}
	}

	close(m.stopCh)
}

// resolvePort returns the project's assigned port if it exists and is available,
// otherwise selects a new port and persists it on the project.
func (m *AppManager) resolvePort(ctx context.Context, projectID string) (int, error) {
	assigned, err := m.repo.GetProjectPort(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("get project port: %w", err)
	}

	if assigned != nil {
		// Check if the assigned port is available.
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", *assigned))
		if err == nil {
			ln.Close()
			// Also verify it's not in use by another of our apps.
			inUse := false
			for _, app := range m.apps {
				if app.Port == *assigned {
					inUse = true
					break
				}
			}
			if !inUse {
				return *assigned, nil
			}
		}
		m.logger.Warn("assigned port unavailable, selecting new port",
			"project_id", projectID, "assigned_port", *assigned)
	}

	// No assigned port or it's unavailable — pick a new one and persist it.
	port, err := m.selectPort()
	if err != nil {
		return 0, err
	}
	if err := m.repo.SetProjectPort(ctx, projectID, port); err != nil {
		return 0, fmt.Errorf("persist port: %w", err)
	}
	m.logger.Info("assigned new port to project", "project_id", projectID, "port", port)
	return port, nil
}

func (m *AppManager) selectPort() (int, error) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	portRange := m.portRangeMax - m.portRangeMin

	for i := 0; i < 10; i++ {
		port := m.portRangeMin + rng.Intn(portRange)

		// Check if port is available.
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue // Port in use.
		}
		ln.Close()

		// Check it's not already assigned to one of our apps.
		inUse := false
		for _, app := range m.apps {
			if app.Port == port {
				inUse = true
				break
			}
		}
		if inUse {
			continue
		}

		return port, nil
	}
	return 0, fmt.Errorf("failed to find available port after 10 attempts")
}
