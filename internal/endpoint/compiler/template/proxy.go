package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

const healthPath = "/__health"

// proxyRevealingHeaders lists headers that must never appear in responses.
var proxyRevealingHeaders = map[string]bool{
	"Server":            true,
	"Via":               true,
	"X-Forwarded-For":   true,
	"X-Forwarded-Proto": true,
	"X-Forwarded-Host":  true,
	"X-Forwarded-Port":  true,
	"X-Proxy":           true,
	"X-Proxy-By":        true,
	"X-Powered-By":      true,
	"X-Backend-Server":  true,
}

// transparentProxy is a reverse proxy that forwards HTTP/HTTPS requests
// transparently without adding proxy-revealing headers.
type transparentProxy struct {
	upstreamAddr      string
	transport         *http.Transport
	healthHandler     http.Handler
	shutdownCh        chan struct{}
	shutdownOnce      sync.Once
	activeConnections sync.WaitGroup
}

// newTransparentProxy creates a new transparent reverse proxy.
func newTransparentProxy(upstreamHost, upstreamPort string) *transparentProxy {
	addr := net.JoinHostPort(upstreamHost, upstreamPort)
	if upstreamHost == "" {
		addr = net.JoinHostPort("127.0.0.1", upstreamPort)
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * secondDuration,
			KeepAlive: 30 * secondDuration,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * secondDuration,
	}

	proxy := &transparentProxy{
		upstreamAddr: addr,
		transport:    transport,
		shutdownCh:   make(chan struct{}),
	}

	proxy.healthHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	return proxy
}

// ServeHTTP implements http.Handler for the proxy.
func (p *transparentProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.handleRequest(w, r)
}

// handleRequest is the main request handler for both HTTP and HTTPS.
func (p *transparentProxy) handleRequest(w http.ResponseWriter, r *http.Request) {
	p.activeConnections.Add(1)
	defer p.activeConnections.Done()

	select {
	case <-p.shutdownCh:
		http.Error(w, "service shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	if r.URL.Path == healthPath {
		p.healthHandler.ServeHTTP(w, r)
		return
	}

	if isWebSocketUpgrade(r) {
		p.handleWebSocket(w, r)
		return
	}

	upstreamURL := fmt.Sprintf("http://%s%s", p.upstreamAddr, r.URL.RequestURI())

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		log.Printf("error creating upstream request: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, v := range values {
			upReq.Header.Add(key, v)
		}
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		upReq.Header.Set("X-Real-IP", ip)
	} else if r.RemoteAddr != "" {
		upReq.Header.Set("X-Real-IP", r.RemoteAddr)
	}

	resp, err := p.transport.RoundTrip(upReq)
	if err != nil {
		log.Printf("error forwarding to upstream: %v", err)
		http.Error(w, "upstream connection failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)

	if resp.Body != nil {
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Printf("error streaming response: %v", err)
		}
	}
}

// copyResponseHeaders copies upstream headers to the client, stripping proxy-revealing ones.
func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		if proxyRevealingHeaders[key] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

// Shutdown signals the proxy to begin graceful shutdown.
func (p *transparentProxy) Shutdown() {
	p.shutdownOnce.Do(func() {
		close(p.shutdownCh)
	})
}
