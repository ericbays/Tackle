package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	connection := strings.ToLower(r.Header.Get("Connection"))
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(connection, "upgrade") && upgrade == "websocket"
}

// handleWebSocket handles a WebSocket upgrade request by establishing a transparent
// bidirectional proxy between the client and the upstream server.
func (p *transparentProxy) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	p.activeConnections.Add(1)
	defer p.activeConnections.Done()

	// Hijack the client connection to get raw TCP access.
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("websocket: response writer does not support hijacking")
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		log.Printf("websocket: hijack failed: %v", err)
		http.Error(w, "hijack failed", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Dial the upstream server via raw TCP.
	upstreamConn, err := net.DialTimeout("tcp", p.upstreamAddr, 30*secondDuration)
	if err != nil {
		log.Printf("websocket: upstream dial failed: %v", err)
		clientBuf.WriteString("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n")
		clientBuf.Flush()
		return
	}
	defer upstreamConn.Close()

	// Build and write the upgrade request to upstream.
	upReq := &http.Request{
		Method:     r.Method,
		URL:        r.URL,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     r.Header.Clone(),
		Host:       p.upstreamAddr,
	}

	if err := upReq.Write(upstreamConn); err != nil {
		log.Printf("websocket: failed to write upgrade request: %v", err)
		return
	}

	// Read the upstream's 101 response.
	upstreamBuf := bufio.NewReader(upstreamConn)
	upstreamResp, err := http.ReadResponse(upstreamBuf, upReq)
	if err != nil {
		log.Printf("websocket: failed to read upstream response: %v", err)
		return
	}

	if upstreamResp.StatusCode != http.StatusSwitchingProtocols {
		log.Printf("websocket: upstream returned %d, expected 101", upstreamResp.StatusCode)
		// Forward the non-101 response to client as-is.
		upstreamResp.Write(clientConn)
		return
	}

	// Write the 101 response back to the client via the hijacked connection.
	respLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", upstreamResp.StatusCode, http.StatusText(upstreamResp.StatusCode))
	clientBuf.WriteString(respLine)
	for key, values := range upstreamResp.Header {
		if proxyRevealingHeaders[key] {
			continue
		}
		for _, v := range values {
			clientBuf.WriteString(fmt.Sprintf("%s: %s\r\n", key, v))
		}
	}
	clientBuf.WriteString("\r\n")
	clientBuf.Flush()

	// Bidirectional copy. Block until one direction closes.
	done := make(chan struct{})

	// Client → upstream.
	go func() {
		io.Copy(upstreamConn, clientConn)
		// When client closes, half-close upstream write side if possible.
		if tc, ok := upstreamConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
		close(done)
	}()

	// Upstream → client (run on this goroutine so defers keep connections alive).
	io.Copy(clientConn, upstreamConn)
	// When upstream closes, half-close client write side.
	if tc, ok := clientConn.(*net.TCPConn); ok {
		tc.CloseWrite()
	}

	// Wait for client→upstream to finish too.
	<-done
}
