package servergen

import (
	"fmt"
	"strings"
)

// GenerateBackend processes the definitions and statically maps the server structure into hardcoded Go files
func GenerateBackend(projectID, buildID string, definition map[string]any, isDevelopment bool) (map[string]string, error) {
	files := make(map[string]string)

	devConstants := ""
	if isDevelopment {
		devConstants = fmt.Sprintf(`
	isDevelopment = true
	projectID = "%s"
	buildID = "%s"
`, projectID, buildID)
	}

	// 1. main.go
	files["main.go"] = fmt.Sprintf(`package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var _ = bytes.NewBuffer
var _ = json.Marshal
var _ = fmt.Printf

var (
	isDevelopment = false
	projectID     = ""
	buildID       = ""
	backendPort   = 0
)

func init() {
	%s
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil { return 0, err }
	l, err := net.ListenTCP("tcp", addr)
	if err != nil { return 0, err }
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func main() {
	fmt.Println("Starting Standalone Landing Binary...")
	
	// Boot Router
	mux := setupRouter()

	port, err := getFreePort()
	if err != nil {
		log.Fatalf("Failed to allocate port: %%v", err)
	}
	backendPort = port

	serverURL := fmt.Sprintf("127.0.0.1:%%d", port)
	srv := &http.Server{Addr: serverURL, Handler: mux}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %%v", err)
		}
	}()

	fmt.Printf("Server listening on %%s\n", serverURL)

	if isDevelopment {
		// Register with Tackle DevServerManager bottom-up Webhook!
		payload, _ := json.Marshal(map[string]interface{}{
			"project_id": projectID,
			"build_id":   buildID,
			"port_a":     port,
			// Since we use no separate local React dev server inside the binary yet (esbuild pre-bundle deployed) port_b handles everything
			"port_b":     port, 
			"mode":       "development",
			"pid":        os.Getpid(),
		})
		
		// Attempt registration to framework running natively on 8080 or port from TACKLE_HOST
		tackleHost := os.Getenv("TACKLE_HOST")
		if tackleHost == "" {
			tackleHost = "http://127.0.0.1:8080"
		}
		
		regURL := tackleHost + "/api/v1/internal/dev-server/register"
		resp, err := http.Post(regURL, "application/json", bytes.NewBuffer(payload))
		if err != nil || resp.StatusCode != 200 {
			log.Fatalf("Failed to register with Tackle Framework at %%s: %%v", regURL, err)
		}
		
		fmt.Println("Successfully registered with Tackle Framework. Starting Heartbeats...")

		// Kick off heartbeat daemon
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			for range ticker.C {
				hbPayload, _ := json.Marshal(map[string]interface{}{
					"project_id": projectID,
					"build_id":   buildID,
					"uptime_seconds": 10,
				})
				hbURL := tackleHost + "/api/v1/internal/dev-server/heartbeat"
				if _, err := http.Post(hbURL, "application/json", bytes.NewBuffer(hbPayload)); err != nil {
					log.Printf("Heartbeat error: %%v", err)
				}
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down binary...")
}
`, devConstants)


	// 2. server.go - Setup router and embed the static UI artifacts
	files["server.go"] = `package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

func setupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	var fileServer http.Handler
	if isDevelopment {
		fileServer = http.FileServer(http.Dir("static"))
	} else {
		staticAssets, _ := fs.Sub(staticFS, "static")
		fileServer = http.FileServer(http.FS(staticAssets))
	}
	
	mux.Handle("/assets/", http.StripPrefix("/assets/", fileServer))

	// Map Form Capture Endpoints natively generated from AST
	registerCaptureHandlers(mux)

	// Catch-all React Router Fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Do not catch /api or strictly raw API paths inside catch-all
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		
		// Map everything else to the injected JS payload HTML shell
		var fileData []byte
		var err error
		if isDevelopment {
			fileData, err = os.ReadFile("static/index.html")
		} else {
			fileData, err = staticFS.ReadFile("static/index.html")
		}

		if err != nil {
			http.Error(w, "Fatal: index.html missing from binary", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(fileData)
	})

	return mux
}
`

	// 3. handlers.go - Scour the AST for all forms and dynamically write exact `POST` mappings
	var handlerBuilder strings.Builder
	handlerBuilder.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

var _ = json.NewDecoder
var _ = fmt.Printf

func registerCaptureHandlers(mux *http.ServeMux) {
`)

	// Harvest unique actions from all forms across all pages
	actions := make(map[string]bool)
	if p, ok := definition["pages"].([]any); ok {
		for _, pi := range p {
			if pm, ok := pi.(map[string]any); ok {
				tree := getList(pm, "component_tree")
				harvestFormActions(tree, actions)
			}
		}
	}

	for action := range actions {
		handlerBuilder.WriteString(fmt.Sprintf(`
	mux.HandleFunc("POST %s", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		
		// In a production compilation, here is where we format CaptureEvents
		// and blindly relay them using the non-authenticated local internal bridge.
		fmt.Printf("Received Submission on %s: %%+v\n", payload)
		
		// Dynamic redirect block mapped back to client
		response := map[string]string{
			"status": "captured",
			// "redirect": "https://google.com", // Dynamic payload based on AST 
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
`, action, action))
	}

	handlerBuilder.WriteString("}\n")
	files["handlers.go"] = handlerBuilder.String()

	return files, nil
}

// harvestFormActions recurses DOM structures to extract routing behavior points out of the raw AST
func harvestFormActions(tree []map[string]any, actions map[string]bool) {
	for _, node := range tree {
		if getString(node, "type") == "form" {
			props := getMap(node, "properties")
			action := getString(props, "action")
			if action == "" {
				action = "/api/submit"
			}
			actions[action] = true
		}
		if children := getList(node, "children"); len(children) > 0 {
			harvestFormActions(children, actions)
		}
	}
}
