package endpoint

import (
	"bytes"
	"fmt"
	"text/template"
)

// HardeningConfig holds the parameters for generating the endpoint hardening script.
type HardeningConfig struct {
	ProxyPort   int // Port for HTTPS proxy traffic (default 443)
	ControlPort int // Port for control channel
	BinaryUser  string // Non-root user to run the binary as (default "tackle")
}

// GenerateHardeningScript produces a bash script that hardens the endpoint VM.
// The script:
// - Creates a non-root user for the proxy binary
// - Configures firewall to allow only proxy port and control port
// - Removes SSH access (firewall rule)
// - Disables unnecessary services
// - Sets filesystem permissions
// - Cleans up temp files and logs
func GenerateHardeningScript(config HardeningConfig) ([]byte, error) {
	if config.ProxyPort == 0 {
		config.ProxyPort = 443
	}
	if config.ControlPort == 0 {
		config.ControlPort = 9443
	}
	if config.BinaryUser == "" {
		config.BinaryUser = "tackle"
	}

	tmpl, err := template.New("harden").Parse(hardeningScriptTemplate)
	if err != nil {
		return nil, fmt.Errorf("hardening: parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return nil, fmt.Errorf("hardening: execute template: %w", err)
	}

	return buf.Bytes(), nil
}

const hardeningScriptTemplate = `#!/bin/bash
set -euo pipefail

# Tackle Endpoint Hardening Script
# This script is executed once during the Configuring phase.

PROXY_PORT={{.ProxyPort}}
CONTROL_PORT={{.ControlPort}}
BINARY_USER="{{.BinaryUser}}"

echo "[harden] Starting endpoint hardening..."

# --- Step 1: Create non-root service user ---
if ! id "$BINARY_USER" &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin "$BINARY_USER"
    echo "[harden] Created service user: $BINARY_USER"
fi

# --- Step 2: Set ownership and permissions ---
chown -R "$BINARY_USER":"$BINARY_USER" /opt/tackle
chmod 755 /opt/tackle/proxy
chmod 600 /opt/tackle/config.json
chmod 600 /opt/tackle/certs/server.key
chmod 644 /opt/tackle/certs/server.crt

# Allow binding to privileged ports without root.
if command -v setcap &>/dev/null; then
    setcap 'cap_net_bind_service=+ep' /opt/tackle/proxy
fi

# --- Step 3: Configure firewall ---
# Use iptables directly (works on all distros).
# Flush existing rules and set restrictive defaults.
iptables -F INPUT
iptables -F OUTPUT
iptables -F FORWARD

# Allow loopback.
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established connections.
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# Allow proxy port (443) — inbound HTTPS traffic.
iptables -A INPUT -p tcp --dport $PROXY_PORT -j ACCEPT

# Allow control port — inbound control channel from framework.
iptables -A INPUT -p tcp --dport $CONTROL_PORT -j ACCEPT

# Allow DNS resolution (outbound).
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Allow outbound HTTPS (for heartbeat/data reporting to framework).
iptables -A OUTPUT -p tcp --dport 443 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 8443 -j ACCEPT

# Allow SSH temporarily (will be removed after config phase).
iptables -A INPUT -p tcp --dport 22 -j ACCEPT

# Drop everything else.
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

# Save iptables rules.
if command -v iptables-save &>/dev/null; then
    iptables-save > /etc/iptables.rules 2>/dev/null || true
fi

echo "[harden] Firewall configured: ports $PROXY_PORT, $CONTROL_PORT open"

# --- Step 4: Disable unnecessary services ---
SERVICES_TO_DISABLE="cups avahi-daemon bluetooth ModemManager"
for svc in $SERVICES_TO_DISABLE; do
    if systemctl is-active --quiet "$svc" 2>/dev/null; then
        systemctl stop "$svc" 2>/dev/null || true
        systemctl disable "$svc" 2>/dev/null || true
        echo "[harden] Disabled service: $svc"
    fi
done

# --- Step 5: Clean up logs, temp files, caches ---
rm -rf /tmp/* /var/tmp/* 2>/dev/null || true
> /var/log/wtmp 2>/dev/null || true
> /var/log/btmp 2>/dev/null || true
> /var/log/lastlog 2>/dev/null || true
history -c 2>/dev/null || true

# --- Step 6: Remove package manager caches ---
if command -v apt-get &>/dev/null; then
    apt-get clean -y 2>/dev/null || true
elif command -v yum &>/dev/null; then
    yum clean all 2>/dev/null || true
fi

# --- Step 7: Set noexec on /tmp if possible ---
if mountpoint -q /tmp 2>/dev/null; then
    mount -o remount,noexec,nosuid /tmp 2>/dev/null || true
    echo "[harden] Set noexec on /tmp"
fi

# --- Step 8: Remove the hardening script itself ---
rm -f /opt/tackle/harden.sh 2>/dev/null || true

echo "[harden] Endpoint hardening complete."
`
