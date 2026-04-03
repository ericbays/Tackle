package endpoint

import (
	"strings"
	"testing"
)

func TestGenerateHardeningScriptDefaults(t *testing.T) {
	script, err := GenerateHardeningScript(HardeningConfig{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s := string(script)

	checks := []struct {
		name string
		want string
	}{
		{"shebang", "#!/bin/bash"},
		{"set flags", "set -euo pipefail"},
		{"default proxy port", "PROXY_PORT=443"},
		{"default control port", "CONTROL_PORT=9443"},
		{"default user", "BINARY_USER=\"tackle\""},
		{"create user", "useradd --system"},
		{"iptables flush", "iptables -F INPUT"},
		{"allow 443", "--dport $PROXY_PORT -j ACCEPT"},
		{"allow control", "--dport $CONTROL_PORT -j ACCEPT"},
		{"drop input default", "iptables -P INPUT DROP"},
		{"drop forward default", "iptables -P FORWARD DROP"},
		{"disable cups", "cups"},
		{"chown tackle", "chown -R \"$BINARY_USER\""},
		{"chmod binary", "chmod 755 /opt/tackle/proxy"},
		{"chmod config", "chmod 600 /opt/tackle/config.json"},
		{"chmod tls key", "chmod 600 /opt/tackle/certs/server.key"},
		{"no-new-privileges via setcap", "setcap"},
		{"cleanup tmp", "rm -rf /tmp/*"},
		{"noexec tmp", "noexec"},
		{"self-destruct", "rm -f /opt/tackle/harden.sh"},
	}
	for _, tc := range checks {
		if !strings.Contains(s, tc.want) {
			t.Errorf("[%s] script missing: %s", tc.name, tc.want)
		}
	}
}

func TestGenerateHardeningScriptCustom(t *testing.T) {
	script, err := GenerateHardeningScript(HardeningConfig{
		ProxyPort:   8443,
		ControlPort: 7443,
		BinaryUser:  "proxy-svc",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s := string(script)

	if !strings.Contains(s, "PROXY_PORT=8443") {
		t.Error("should use custom proxy port")
	}
	if !strings.Contains(s, "CONTROL_PORT=7443") {
		t.Error("should use custom control port")
	}
	if !strings.Contains(s, "BINARY_USER=\"proxy-svc\"") {
		t.Error("should use custom binary user")
	}
}

func TestGenerateHardeningScriptFirewallRules(t *testing.T) {
	script, err := GenerateHardeningScript(HardeningConfig{ProxyPort: 443, ControlPort: 9443})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s := string(script)

	// Verify essential firewall rules.
	rules := []string{
		"iptables -A INPUT -i lo -j ACCEPT",
		"iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT",
		"iptables -A INPUT -p tcp --dport $PROXY_PORT -j ACCEPT",
		"iptables -A INPUT -p tcp --dport $CONTROL_PORT -j ACCEPT",
		"iptables -P INPUT DROP",
		"iptables -P FORWARD DROP",
		"iptables -P OUTPUT DROP",
	}
	for _, rule := range rules {
		if !strings.Contains(s, rule) {
			t.Errorf("missing firewall rule: %s", rule)
		}
	}
}

func TestGenerateHardeningScriptSSHTemporarilyAllowed(t *testing.T) {
	script, err := GenerateHardeningScript(HardeningConfig{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s := string(script)

	// SSH should be temporarily allowed (removed later by SSHDeployer after config).
	if !strings.Contains(s, "--dport 22 -j ACCEPT") {
		t.Error("SSH should be temporarily allowed during hardening")
	}
}

func TestGenerateHardeningScriptNoRootExecution(t *testing.T) {
	script, err := GenerateHardeningScript(HardeningConfig{BinaryUser: "tackle"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s := string(script)

	if !strings.Contains(s, "useradd --system --no-create-home --shell /usr/sbin/nologin") {
		t.Error("should create system user with nologin shell")
	}
}
