package landingpages

import (
	"encoding/json"
	"net/http"
	"sync"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for dev
	},
}

// ContextEngine handles distributing AST changes via HMR WebSockets
type ContextEngine struct {
	mu          sync.RWMutex
	connections map[string]map[*websocket.Conn]bool
}

var (
	ceInstance *ContextEngine
	ceOnce     sync.Once
)

func GetContextEngine() *ContextEngine {
	ceOnce.Do(func() {
		ceInstance = &ContextEngine{
			connections: make(map[string]map[*websocket.Conn]bool),
		}
	})
	return ceInstance
}

// HMRProvider handles incoming websocket connections from DevModeProviders
func (d *Deps) HMRProvider(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	
	ce := GetContextEngine()
	ce.mu.Lock()
	if ce.connections[projectID] == nil {
		ce.connections[projectID] = make(map[*websocket.Conn]bool)
	}
	ce.connections[projectID][conn] = true
	ce.mu.Unlock()

	defer func() {
		ce.mu.Lock()
		delete(ce.connections[projectID], conn)
		conn.Close()
		ce.mu.Unlock()
	}()

	// Keep alive loop
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// PushASTUpdate sends an AST down the socket to active dev server wrappers
func (ce *ContextEngine) PushASTUpdate(projectID string, astData interface{}) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if conns, exists := ce.connections[projectID]; exists {
		for conn := range conns {
			_ = conn.WriteJSON(astData)
		}
	}
}

// HandleASTPush consumes the JSON payload over HTTP and forwards it to the ContextEngine
func (d *Deps) HandleASTPush(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		http.Error(w, "missing project id", http.StatusBadRequest)
		return
	}

	var payload interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	GetContextEngine().PushASTUpdate(projectID, payload)
	
	// Fast stateless response
	w.WriteHeader(http.StatusAccepted)
}
