package landingpages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"tackle/internal/compiler/devserver"
	"tackle/internal/compiler/servergen"
	"tackle/internal/middleware"
	"tackle/pkg/response"
)

// StartDevServer handles the deployment of a development build
func (d *Deps) StartDevServer(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		response.Error(w, "BAD_REQUEST", "Missing campaign ID", http.StatusBadRequest, correlationID)
		return
	}

	// Fetch actual AST for structural compilation
	project, err := d.Svc.Get(r.Context(), campaignID)
	if err != nil {
		response.Error(w, "INTERNAL_ERROR", "Failed to load project AST", http.StatusInternalServerError, correlationID)
		return
	}

	// 0. Ensure any existing development server is formally killed to release OS executable file locks
	mgr := devserver.GetManager()
	mgr.StopDevServer(campaignID)
	// Give Windows process monitor 100ms to organically unlock the binary
	time.Sleep(100 * time.Millisecond)

	// 1. Generate dev server workspace natively
	tmpDir := filepath.Join(os.TempDir(), "tackle-dev-builds", campaignID)
	os.RemoveAll(tmpDir) // Reset pristine workspace
	os.MkdirAll(tmpDir, 0755)
	
	// Delegate generation to the servergen pipeline specifically wired for Development
	_, err = servergen.GenerateWorkspace(tmpDir, campaignID, "dev-build", project.DefinitionJSON, true)
	if err != nil {
		fmt.Println("!!!! SERVERGEN FAILED !!!!", err.Error())
		os.WriteFile("compiler-error.txt", []byte("SERVERGEN FAILED: "+err.Error()), 0644)
		response.Error(w, "INTERNAL_ERROR", "Servergen failed: "+err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	// 2. Synchronous Go Build Execution ensuring unique output binary names to bypass any lingering Windows OS file locks
	buildPath := filepath.Join(tmpDir, fmt.Sprintf("dev-binary-%d", time.Now().UnixNano()))
	if runtime.GOOS == "windows" {
		buildPath += ".exe"
	}
	
	cmd := exec.Command("go", "build", "-o", buildPath, ".")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println("!!!! GO COMPILER FAILED !!!!", string(out))
		os.WriteFile("compiler-error.txt", []byte("GO COMPILER FAILED: "+string(out)+"\nERR: "+err.Error()), 0644)
		response.Error(w, "INTERNAL_ERROR", "Go compiler failed: "+string(out), http.StatusInternalServerError, correlationID)
		return
	}

	err = mgr.StartDevServer(campaignID, buildPath)
	if err != nil {
		fmt.Println("!!!! START DEVSERVER FAILED !!!!", err.Error())
		response.Error(w, "INTERNAL_ERROR", "Failed to start dev server: "+err.Error(), http.StatusInternalServerError, correlationID)
		return
	}

	response.Success(w, map[string]interface{}{
		"status": "starting",
	})
}

// StopDevServer handles terminating the development server
func (d *Deps) StopDevServer(w http.ResponseWriter, r *http.Request) {
	correlationID := middleware.GetCorrelationID(r.Context())
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		response.Error(w, "BAD_REQUEST", "Missing campaign ID", http.StatusBadRequest, correlationID)
		return
	}

	mgr := devserver.GetManager()
	mgr.StopDevServer(campaignID)

	response.Success(w, map[string]interface{}{
		"status": "offline",
	})
}

// GetDevServerStatus returns the persistent state of the dev server for hydration
func (d *Deps) GetDevServerStatus(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	if campaignID == "" {
		// Just silently return offline instead of cluttering logs since polling handles it
		response.Success(w, map[string]interface{}{"status": "offline"})
		return
	}

	mgr := devserver.GetManager()
	p := mgr.GetDevServerStatus(campaignID)

	if p != nil {
		resp := map[string]interface{}{
			"status":         p.Status,
			"uptime_seconds": int(time.Since(p.StartTime).Seconds()),
		}
		if p.Status == "online" {
			resp["port_a"] = p.PortA
			resp["port_b"] = p.PortB
			resp["url_a"] = "http://127.0.0.1:" + strconv.Itoa(p.PortA)
			resp["url_b"] = "http://127.0.0.1:" + strconv.Itoa(p.PortB)
			resp["build_id"] = p.BuildID
		}
		response.Success(w, resp)
	} else {
		response.Success(w, map[string]interface{}{
			"status": "offline",
		})
	}
}

// DevServerRegistrationRequest represents the payload from child dev binary
type DevServerRegistrationRequest struct {
	ProjectID string `json:"project_id"`
	BuildID   string `json:"build_id"`
	PortA     int    `json:"port_a"`
	PortB     int    `json:"port_b"`
	Mode      string `json:"mode"`
	PID       int    `json:"pid"`
}

// HandleRegisterDevServer accepts bottom-up registration
func (d *Deps) HandleRegisterDevServer(w http.ResponseWriter, r *http.Request) {
	var req DevServerRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "Invalid body", http.StatusBadRequest, "")
		return
	}

	if req.ProjectID == "" || req.PortA == 0 || req.PortB == 0 {
		response.Error(w, "BAD_REQUEST", "Missing required fields", http.StatusBadRequest, "")
		return
	}

	mgr := devserver.GetManager()
	if err := mgr.Register(req.ProjectID, req.BuildID, req.PortA, req.PortB, req.PID); err != nil {
		response.Error(w, "NOT_FOUND", err.Error(), http.StatusNotFound, "")
		return
	}

	response.Success(w, map[string]string{"status": "registered"})
}

type DevServerHeartbeatRequest struct {
	ProjectID     string `json:"project_id"`
	BuildID       string `json:"build_id"`
	PortA         int    `json:"port_a"`
	PortB         int    `json:"port_b"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

func (d *Deps) HandleHeartbeatDevServer(w http.ResponseWriter, r *http.Request) {
	var req DevServerHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "Invalid body", http.StatusBadRequest, "")
		return
	}

	mgr := devserver.GetManager()
	if err := mgr.RecordHeartbeat(req.ProjectID); err != nil {
		// Child app should terminate itself if heartbeat returns 404
		response.Error(w, "NOT_FOUND", err.Error(), http.StatusNotFound, "")
		return
	}

	response.Success(w, map[string]string{"status": "ok"})
}

type DevServerDeregisterRequest struct {
	ProjectID string `json:"project_id"`
	BuildID   string `json:"build_id"`
	Reason    string `json:"reason"`
}

func (d *Deps) HandleDeregisterDevServer(w http.ResponseWriter, r *http.Request) {
	var req DevServerDeregisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "BAD_REQUEST", "Invalid body", http.StatusBadRequest, "")
		return
	}

	mgr := devserver.GetManager()
	mgr.StopDevServer(req.ProjectID)

	response.Success(w, map[string]string{"status": "deregistered"})
}

// SyncDevServer handles real-time HMR payload syncing from the UI without fully rebuilding the binary
func (d *Deps) SyncDevServer(w http.ResponseWriter, r *http.Request) {
	campaignID := chi.URLParam(r, "id")
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Overwrite the existing React static build via esbuild transpilation
	tmpDir := filepath.Join(os.TempDir(), "tackle-dev-builds", campaignID)
	err := servergen.GenerateFrontendOnly(tmpDir, payload, true)
	if err != nil {
		fmt.Println("!!!! HMR FRONTEND COMPILE FAILED !!!!", err.Error())
		http.Error(w, "HMR Compilation Failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Trigger the HMR broadcast to active connected preview frames
	GetContextEngine().PushASTUpdate(campaignID, "reload")

	w.WriteHeader(http.StatusAccepted)
}
