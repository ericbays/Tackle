package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"
)

// BuildInfo holds the deployment-specific information embedded at build time.
type BuildInfo struct {
	CampaignID      string
	EndpointID      string
	DeployTimestamp int64
	BuildNonce      string
	FrameworkHost   string
	LandingPagePort string
	ControlPort     string
	AuthToken       string
}

// These variables are injected at build time via -ldflags.
var (
	campaignID      string
	endpointID      string
	deployTimestamp string
	buildNonce      string
	frameworkHost   string
	landingPagePort string
	controlPort     string
	authToken       string
)

// BuildInfoAtRuntime creates a BuildInfo struct using the injected variables.
func BuildInfoAtRuntime() BuildInfo {
	var ts time.Time
	var nsec int64
	if _, err := fmt.Sscanf(deployTimestamp, "%d", &nsec); err == nil && nsec > 0 {
		ts = time.Unix(0, nsec)
	} else {
		ts = time.Now()
	}

	return BuildInfo{
		CampaignID:      campaignID,
		EndpointID:      endpointID,
		DeployTimestamp: ts.UnixNano(),
		BuildNonce:      buildNonce,
		FrameworkHost:   frameworkHost,
		LandingPagePort: landingPagePort,
		ControlPort:     controlPort,
		AuthToken:       authToken,
	}
}

// ComputeBinaryFingerprint generates a unique fingerprint based on build variables.
func ComputeBinaryFingerprint() string {
	h := sha256.New()
	if len(campaignID) >= 8 {
		h.Write([]byte(campaignID[:8]))
	} else {
		h.Write([]byte(campaignID))
	}
	if len(endpointID) >= 8 {
		h.Write([]byte(endpointID[:8]))
	} else {
		h.Write([]byte(endpointID))
	}
	if len(deployTimestamp) > 0 {
		h.Write([]byte(deployTimestamp))
	}
	h.Write([]byte(buildNonce))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// generateEntropy creates additional runtime entropy from the system.
func generateEntropy() string {
	var buf [32]byte
	if _, err := cryptoRandRead(buf[:]); err != nil {
		log.Printf("warning: failed to read entropy: %v", err)
	}
	return hex.EncodeToString(buf[:])
}

// getPort returns the numeric port from a port string (handles ":port" format).
func getPort(portStr string) uint16 {
	if len(portStr) > 0 && portStr[0] == ':' {
		portStr = portStr[1:]
	}
	var port uint16
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		port = 8080
	}
	return port
}
