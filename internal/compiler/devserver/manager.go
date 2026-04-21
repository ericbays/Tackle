package devserver

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// DevProcess tracks a running development server instance
type DevProcess struct {
	Cmd           *exec.Cmd
	ProjectID     string
	BuildID       string
	PortA         int    // Go Backend
	PortB         int    // React Dev Server
	Status        string // "pending", "online", "offline"
	PID           int
	StartTime     time.Time
	LastHeartbeat time.Time
}

type DevServerManager struct {
	mu        sync.RWMutex
	processes map[string]*DevProcess
}

var (
	instance *DevServerManager
	once     sync.Once
)

// GetManager returns the singleton instance of the DevServerManager
func GetManager() *DevServerManager {
	once.Do(func() {
		instance = &DevServerManager{
			processes: make(map[string]*DevProcess),
		}
		// Start background tracking for heartbeat monitoring
		go instance.startHeartbeatMonitor()
	})
	return instance
}

// StartDevServer executes the compiled artifact without allocating ports.
// The child process is expected to POST /api/v1/internal/dev-server/register once it boots.
func (m *DevServerManager) StartDevServer(projectID string, buildPath string, buildDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Terminate existing server for this project to ensure 1:1 mapping
	if existing, exists := m.processes[projectID]; exists {
		m.killProcess(existing)
		delete(m.processes, projectID)
	}

	cmd := exec.Command(buildPath)
	cmd.Dir = buildDir
	cmd.Env = append(cmd.Environ(), "ENV=development")
	
	logFile, _ := os.Create(buildDir + "/dev-stderr.log")
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to spawn dev server: %w", err)
	}

	m.processes[projectID] = &DevProcess{
		Cmd:           cmd,
		ProjectID:     projectID,
		Status:        "pending",
		PID:           cmd.Process.Pid,
		StartTime:     time.Now(),
		LastHeartbeat: time.Now(),
	}

	// Wait in a separate goroutine to reap the process
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// Register is called when the child POSTs its webhook payload back to Tackle
func (m *DevServerManager) Register(projectID string, buildID string, portA int, portB int, pid int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.processes[projectID]
	if !exists {
		return fmt.Errorf("no pending process found for project %s", projectID)
	}

	process.BuildID = buildID
	process.PortA = portA
	process.PortB = portB
	if pid > 0 {
		process.PID = pid
	}
	process.Status = "online"
	process.LastHeartbeat = time.Now()

	return nil
}

// RecordHeartbeat updates the last seen timestamp
func (m *DevServerManager) RecordHeartbeat(projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.processes[projectID]
	if !exists {
		return fmt.Errorf("process not found")
	}

	process.LastHeartbeat = time.Now()
	return nil
}

// StopDevServer explicitly kills the running server for the project
func (m *DevServerManager) StopDevServer(projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if process, exists := m.processes[projectID]; exists {
		m.killProcess(process)
		delete(m.processes, projectID)
	}
}

// killProcess sends a termination signal to the tracked binary
func (m *DevServerManager) killProcess(process *DevProcess) {
	if process.Cmd != nil && process.Cmd.Process != nil {
		_ = process.Cmd.Process.Kill() // Robust kill
	}
}

// GetDevServerStatus returns a copy of the active process status
func (m *DevServerManager) GetDevServerStatus(projectID string) *DevProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if process, exists := m.processes[projectID]; exists {
		// Verify it hasn't exited naturally
		if process.Cmd != nil && process.Cmd.ProcessState != nil && process.Cmd.ProcessState.Exited() {
			return nil
		}
		// Return a copy so caller doesn't hold direct pointer reference
		pCopy := *process
		return &pCopy
	}
	return nil
}

// startHeartbeatMonitor replaces the old zombie cleanup logic
func (m *DevServerManager) startHeartbeatMonitor() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		m.checkHeartbeats()
	}
}

func (m *DevServerManager) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, p := range m.processes {
		// Cleanup if it naturally exited
		if p.Cmd != nil && p.Cmd.ProcessState != nil && p.Cmd.ProcessState.Exited() {
			delete(m.processes, id)
			continue
		}

		// If it's online and misses 3 heartbeats (30 seconds), terminate
		if p.Status == "online" && now.Sub(p.LastHeartbeat) > 30*time.Second {
			m.killProcess(p)
			delete(m.processes, id)
			continue
		}

		// If it's pending for more than 30 seconds, it failed to boot/register natively
		if p.Status == "pending" && now.Sub(p.StartTime) > 30*time.Second {
			m.killProcess(p)
			delete(m.processes, id)
		}
	}
}
