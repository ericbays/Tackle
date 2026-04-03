package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupTestProxy creates a proxy pointed at a test upstream server.
func setupTestProxy(t *testing.T, upstream *httptest.Server) *transparentProxy {
	t.Helper()
	u, _ := url.Parse(upstream.URL)
	return newTransparentProxy(u.Hostname(), u.Port())
}

// --- HTTP proxy tests ---

func TestAllHTTPMethodsProxied(t *testing.T) {
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodPatch, http.MethodOptions, http.MethodHead,
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != method {
					t.Errorf("upstream got method %s, want %s", r.Method, method)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("proxied-" + r.Method))
			}))
			defer upstream.Close()

			proxy := setupTestProxy(t, upstream)
			var body io.Reader
			if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
				body = strings.NewReader(`{"test":"data"}`)
			}
			req := httptest.NewRequest(method, "/test-path", body)
			rr := httptest.NewRecorder()
			proxy.handleRequest(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rr.Code)
			}
			if method != http.MethodHead {
				if got := rr.Body.String(); got != "proxied-"+method {
					t.Errorf("body = %q, want %q", got, "proxied-"+method)
				}
			}
		})
	}
}

func TestRequestHeadersForwarded(t *testing.T) {
	var receivedHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("User-Agent", "TestClient/1.0")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept-Language", "en-US")

	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	checks := map[string]string{
		"Authorization": "Bearer token123", "User-Agent": "TestClient/1.0",
		"X-Custom-Header": "custom-value", "Accept-Language": "en-US",
	}
	for k, want := range checks {
		if got := receivedHeaders.Get(k); got != want {
			t.Errorf("upstream header %s = %q, want %q", k, got, want)
		}
	}
}

func TestXRealIPAdded(t *testing.T) {
	var gotIP string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = r.Header.Get("X-Real-IP")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if gotIP != "192.168.1.100" {
		t.Errorf("X-Real-IP = %q, want 192.168.1.100", gotIP)
	}
}

func TestNoProxyRevealingHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "nginx/1.25")
		w.Header().Set("Via", "1.1 proxy.internal")
		w.Header().Set("X-Forwarded-For", "10.0.0.1")
		w.Header().Set("X-Powered-By", "Express")
		w.Header().Set("X-Custom-Safe", "should-pass-through")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	for _, h := range []string{"Server", "Via", "X-Forwarded-For", "X-Powered-By"} {
		if v := rr.Header().Get(h); v != "" {
			t.Errorf("response contains forbidden header %s: %q", h, v)
		}
	}
	if v := rr.Header().Get("X-Custom-Safe"); v != "should-pass-through" {
		t.Errorf("X-Custom-Safe = %q, want should-pass-through", v)
	}
}

func TestNoRedirectInjection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Header().Get("Location") != "" {
		t.Errorf("proxy injected Location header: %s", rr.Header().Get("Location"))
	}
}

func TestUpstreamRedirectPassthrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/new-page")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/old-page", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/new-page" {
		t.Errorf("Location = %q, want /new-page", loc)
	}
}

func TestResponseBodyStreaming(t *testing.T) {
	const bodySize = 256 * 1024
	expectedBody := strings.Repeat("X", bodySize)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, expectedBody)
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Body.Len() != bodySize {
		t.Errorf("body length = %d, want %d", rr.Body.Len(), bodySize)
	}
}

func TestHealthCheckHandledLocally(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should NOT be called for /__health")
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, healthPath, nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.String(); got != `{"status":"ok"}` {
		t.Errorf("body = %q, want {\"status\":\"ok\"}", got)
	}
}

func TestChunkedTransferEncoding(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("no Flusher")
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "chunk-%d\n", i)
			flusher.Flush()
		}
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/chunked", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	body := rr.Body.String()
	for i := 0; i < 5; i++ {
		if !strings.Contains(body, fmt.Sprintf("chunk-%d", i)) {
			t.Errorf("missing chunk-%d in body", i)
		}
	}
}

func TestRequestBodyForwarded(t *testing.T) {
	var receivedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	reqBody := `{"username":"test","password":"hunter2"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(reqBody))
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if receivedBody != reqBody {
		t.Errorf("upstream body = %q, want %q", receivedBody, reqBody)
	}
}

func TestUpstreamDown(t *testing.T) {
	proxy := newTransparentProxy("127.0.0.1", "1")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
}

func TestGracefulShutdown(t *testing.T) {
	proxy := newTransparentProxy("127.0.0.1", "1")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		proxy.activeConnections.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
			proxy.activeConnections.Done()
		}()
	}
	proxy.Shutdown()
	select {
	case <-proxy.shutdownCh:
	default:
		t.Error("shutdownCh was not closed")
	}
	wg.Wait()
}

func TestShutdownIdempotent(t *testing.T) {
	proxy := newTransparentProxy("127.0.0.1", "1")
	proxy.Shutdown()
	proxy.Shutdown()
	proxy.Shutdown()
}

func TestShuttingDownRejectsRequests(t *testing.T) {
	proxy := newTransparentProxy("127.0.0.1", "1")
	proxy.Shutdown()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
}

// --- Build info / utility tests ---

func TestBuildInfoAtRuntime(t *testing.T) {
	campaignID = "test-campaign"
	endpointID = "test-endpoint"
	deployTimestamp = fmt.Sprintf("%d", time.Now().UnixNano())
	buildNonce = "test-nonce"
	frameworkHost = "127.0.0.1:9443"
	landingPagePort = "8443"

	bi := BuildInfoAtRuntime()
	if bi.CampaignID != "test-campaign" {
		t.Errorf("CampaignID = %s, want test-campaign", bi.CampaignID)
	}
	if bi.FrameworkHost != "127.0.0.1:9443" {
		t.Errorf("FrameworkHost = %s, want 127.0.0.1:9443", bi.FrameworkHost)
	}
}

func TestComputeBinaryFingerprint(t *testing.T) {
	campaignID = "campaign-123"
	endpointID = "endpoint-456"
	deployTimestamp = "1234567890"
	buildNonce = "nonce-abc"
	fp := ComputeBinaryFingerprint()
	if len(fp) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(fp))
	}
	if fp2 := ComputeBinaryFingerprint(); fp != fp2 {
		t.Errorf("fingerprint not deterministic")
	}
}

func TestGenerateEntropy(t *testing.T) {
	e := generateEntropy()
	if len(e) != 64 {
		t.Errorf("entropy length = %d, want 64", len(e))
	}
	if e2 := generateEntropy(); e == e2 {
		t.Error("two entropy values should differ")
	}
}

func TestGetPort(t *testing.T) {
	tests := []struct {
		input string
		want  uint16
	}{
		{"8080", 8080}, {":8080", 8080}, {"443", 443}, {":443", 443},
		{"80", 80}, {"", 8080}, {"bad", 8080},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := getPort(tt.input); got != tt.want {
				t.Errorf("getPort(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelfSignedCertGeneration(t *testing.T) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("generateSelfSignedCert: %v", err)
	}
	if cert.Leaf == nil {
		t.Fatal("cert.Leaf is nil")
	}
	if cert.Leaf.NotAfter.Before(time.Now()) {
		t.Error("certificate already expired")
	}
	if cert.Leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("missing DigitalSignature key usage")
	}
}

func TestGetSupportedCipherSuites(t *testing.T) {
	suites := getSupportedCipherSuites()
	if len(suites) == 0 {
		t.Fatal("no cipher suites returned")
	}
	for _, s := range suites {
		name := tls.CipherSuiteName(s)
		if !strings.Contains(name, "ECDHE") {
			t.Errorf("cipher suite %s is not ECDHE", name)
		}
	}
}

func TestNewTransparentProxy(t *testing.T) {
	proxy := newTransparentProxy("127.0.0.1", "8443")
	if proxy.upstreamAddr != "127.0.0.1:8443" {
		t.Errorf("upstreamAddr = %s, want 127.0.0.1:8443", proxy.upstreamAddr)
	}
	if proxy.transport == nil {
		t.Error("transport is nil")
	}
	if proxy.shutdownCh == nil {
		t.Error("shutdownCh is nil")
	}
}

func TestCopyResponseHeaders(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": {"text/html"}, "X-Custom": {"value"},
			"Server": {"nginx"}, "Via": {"1.1 proxy"}, "X-Forwarded-For": {"10.0.0.1"},
		},
	}
	rr := httptest.NewRecorder()
	copyResponseHeaders(rr, resp)

	if v := rr.Header().Get("Content-Type"); v != "text/html" {
		t.Errorf("Content-Type = %q", v)
	}
	if v := rr.Header().Get("X-Custom"); v != "value" {
		t.Errorf("X-Custom = %q", v)
	}
	for _, h := range []string{"Server", "Via", "X-Forwarded-For"} {
		if v := rr.Header().Get(h); v != "" {
			t.Errorf("%s should be stripped, got %q", h, v)
		}
	}
}

// --- WebSocket tests ---

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"standard", map[string]string{"Connection": "Upgrade", "Upgrade": "websocket"}, true},
		{"case variation", map[string]string{"Connection": "UPGRADE", "Upgrade": "WebSocket"}, true},
		{"comma-separated", map[string]string{"Connection": "keep-alive, Upgrade", "Upgrade": "websocket"}, true},
		{"missing Connection", map[string]string{"Upgrade": "websocket"}, false},
		{"missing Upgrade", map[string]string{"Connection": "Upgrade"}, false},
		{"non-websocket", map[string]string{"Connection": "Upgrade", "Upgrade": "h2"}, false},
		{"empty", map[string]string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if got := isWebSocketUpgrade(req); got != tt.want {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestWebSocketUpgradeEndToEnd tests the full WebSocket proxy flow using real TCP connections.
func TestWebSocketUpgradeEndToEnd(t *testing.T) {
	// Start an upstream that does a raw WebSocket upgrade and echoes data.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("upstream does not support hijacking")
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Errorf("upstream hijack: %v", err)
			return
		}
		defer conn.Close()

		bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
		bufrw.WriteString("Upgrade: websocket\r\n")
		bufrw.WriteString("Connection: Upgrade\r\n")
		bufrw.WriteString("\r\n")
		bufrw.Flush()

		// Echo loop.
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return
			}
		}
	}))
	defer upstream.Close()

	// Start the proxy as a real HTTP server so we get real TCP hijacking.
	u, _ := url.Parse(upstream.URL)
	proxy := newTransparentProxy(u.Hostname(), u.Port())
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	// Connect to the proxy via raw TCP and send a WebSocket upgrade.
	pu, _ := url.Parse(proxyServer.URL)
	conn, err := net.DialTimeout("tcp", pu.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	// Send upgrade request.
	upgradeReq := "GET /ws HTTP/1.1\r\n" +
		"Host: " + pu.Host + "\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: websocket\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"\r\n"
	if _, err := conn.Write([]byte(upgradeReq)); err != nil {
		t.Fatalf("write upgrade: %v", err)
	}

	// Read the 101 response.
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read 101 response: %v", err)
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}
	if got := resp.Header.Get("Upgrade"); strings.ToLower(got) != "websocket" {
		t.Errorf("Upgrade header = %q, want websocket", got)
	}

	// Send a message and verify echo.
	message := "hello from test client"
	if _, err := conn.Write([]byte(message)); err != nil {
		t.Fatalf("write message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	echoBuf := make([]byte, len(message))
	n, err := io.ReadFull(reader, echoBuf)
	if err != nil {
		t.Fatalf("read echo: %v (got %d bytes)", err, n)
	}

	if string(echoBuf) != message {
		t.Errorf("echo = %q, want %q", string(echoBuf), message)
	}
}

// TestWebSocketClosePropagation verifies that when the client closes, upstream gets notified.
func TestWebSocketClosePropagation(t *testing.T) {
	upstreamClosed := make(chan struct{})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()

		bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
		bufrw.WriteString("Upgrade: websocket\r\n")
		bufrw.WriteString("Connection: Upgrade\r\n\r\n")
		bufrw.Flush()

		// Read until error (client disconnect).
		buf := make([]byte, 4096)
		for {
			if _, err := conn.Read(buf); err != nil {
				close(upstreamClosed)
				return
			}
		}
	}))
	defer upstream.Close()

	u, _ := url.Parse(upstream.URL)
	proxy := newTransparentProxy(u.Hostname(), u.Port())
	proxyServer := httptest.NewServer(proxy)
	defer proxyServer.Close()

	pu, _ := url.Parse(proxyServer.URL)
	conn, err := net.DialTimeout("tcp", pu.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}

	// Send upgrade.
	upgradeReq := "GET /ws HTTP/1.1\r\nHost: " + pu.Host + "\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n"
	conn.Write([]byte(upgradeReq))

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("read 101: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}

	// Close the client connection.
	conn.Close()

	// Upstream should detect the close.
	select {
	case <-upstreamClosed:
		// Success.
	case <-time.After(3 * time.Second):
		t.Error("upstream did not detect client close within 3s")
	}
}

// TestWebSocketNonWebSocketUnaffected verifies regular HTTP still works with WebSocket support present.
func TestWebSocketNonWebSocketUnaffected(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("regular-response"))
	}))
	defer upstream.Close()
	proxy := setupTestProxy(t, upstream)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rr := httptest.NewRecorder()
	proxy.handleRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "regular-response") {
		t.Errorf("body = %q, want regular-response", rr.Body.String())
	}
}
