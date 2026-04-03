package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startMockSMTP starts a mock SMTP server that accepts connections and responds
// to standard SMTP commands. Returns the address and a cleanup function.
func startMockSMTP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mock smtp listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveMockSMTP(conn)
		}
	}()
	return ln.Addr().String()
}

// serveMockSMTP handles one mock SMTP connection.
func serveMockSMTP(conn net.Conn) {
	defer conn.Close()
	conn.Write([]byte("220 mock-smtp ready\r\n"))

	scanner := bufio.NewScanner(conn)
	inData := false
	for scanner.Scan() {
		line := scanner.Text()
		if inData {
			if line == "." {
				conn.Write([]byte("250 OK\r\n"))
				inData = false
			}
			continue
		}
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch cmd {
		case "EHLO", "HELO":
			conn.Write([]byte("250-mock-smtp\r\n250-AUTH PLAIN LOGIN\r\n250 OK\r\n"))
		case "AUTH":
			conn.Write([]byte("235 Authentication successful\r\n"))
		case "MAIL":
			conn.Write([]byte("250 Sender OK\r\n"))
		case "RCPT":
			conn.Write([]byte("250 Recipient OK\r\n"))
		case "DATA":
			conn.Write([]byte("354 Start data\r\n"))
			inData = true
		case "QUIT":
			conn.Write([]byte("221 Bye\r\n"))
			return
		default:
			conn.Write([]byte("250 OK\r\n"))
		}
	}
}

// newTestRelay creates a relay with an httptest server (no hardcoded port).
func newTestRelay(t *testing.T, authToken string) (*smtpRelay, *httptest.Server) {
	t.Helper()
	relay := newSMTPRelay("test-endpoint", "", authToken, "0")

	mux := http.NewServeMux()
	mux.HandleFunc("/relay/send", relay.handleSend)
	mux.HandleFunc("/relay/cert", relay.handleCertUpdate)
	mux.HandleFunc("/relay/health", relay.handleHealth)
	ts := httptest.NewServer(mux)
	t.Cleanup(func() {
		relay.Stop()
		ts.Close()
	})
	return relay, ts
}

// --- Auth tests ---

func TestSMTPAuthValidation(t *testing.T) {
	_, ts := newTestRelay(t, "secret-token")
	smtpAddr := startMockSMTP(t)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid token", "Bearer secret-token", http.StatusOK},
		{"no header", "", http.StatusUnauthorized},
		{"wrong prefix", "Token secret-token", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := EmailSendCommand{
				ID: "auth-" + tt.name, Recipient: "r@example.com", Subject: "Test",
				BodyHTML: "<p>test</p>", SMTPHost: smtpAddr, SMTPPort: 25,
				EnvelopeSender: "s@example.com",
			}
			body, _ := json.Marshal(cmd)
			req, _ := http.NewRequest(http.MethodPost, ts.URL+"/relay/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// --- Command validation ---

func TestValidateCommand(t *testing.T) {
	relay := newSMTPRelay("ep", "", "", "0")
	tests := []struct {
		name    string
		cmd     *EmailSendCommand
		wantErr string
	}{
		{"valid", &EmailSendCommand{ID: "1", Recipient: "r@e.com", Subject: "S", BodyHTML: "<p>x</p>", SMTPHost: "smtp.e.com"}, ""},
		{"no id", &EmailSendCommand{Recipient: "r@e.com", Subject: "S", BodyHTML: "<p>x</p>", SMTPHost: "smtp.e.com"}, "id is required"},
		{"no recipient", &EmailSendCommand{ID: "1", Subject: "S", BodyHTML: "<p>x</p>", SMTPHost: "smtp.e.com"}, "recipient is required"},
		{"no smtp_host", &EmailSendCommand{ID: "1", Recipient: "r@e.com", Subject: "S", BodyHTML: "<p>x</p>"}, "smtp_host is required"},
		{"no subject", &EmailSendCommand{ID: "1", Recipient: "r@e.com", BodyHTML: "<p>x</p>", SMTPHost: "smtp.e.com"}, "subject is required"},
		{"no body", &EmailSendCommand{ID: "1", Recipient: "r@e.com", Subject: "S", SMTPHost: "smtp.e.com"}, "body_html or body_text is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := relay.validateCommand(tt.cmd)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q doesn't contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- Command parsing via HTTP ---

func TestSMTPCommandParsing(t *testing.T) {
	_, ts := newTestRelay(t, "")
	smtpAddr := startMockSMTP(t)

	tests := []struct {
		name       string
		payload    string
		wantStatus int
	}{
		{"valid", fmt.Sprintf(`{"id":"1","recipient":"r@e.com","subject":"S","body_html":"<p>x</p>","smtp_host":"%s","smtp_port":25,"envelope_sender":"s@e.com"}`, smtpAddr), http.StatusOK},
		{"invalid json", `{"id":"1"`, http.StatusBadRequest},
		{"missing required", `{"id":"1"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/relay/send", "application/json", strings.NewReader(tt.payload))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// --- MIME message building ---

func TestBuildMIMEMessage(t *testing.T) {
	relay := newSMTPRelay("ep", "", "", "0")
	cmd := &EmailSendCommand{
		ID: "mime-1", Recipient: "r@e.com", Subject: "Test Subject",
		BodyHTML: "<html><body>Hello</body></html>", BodyText: "Hello",
		SMTPHost: "smtp.e.com", EnvelopeSender: "s@e.com",
	}
	body, err := relay.buildMIMEMessage(cmd)
	if err != nil {
		t.Fatalf("buildMIMEMessage: %v", err)
	}
	s := string(body)

	for _, want := range []string{"From:", "To:", "Subject:", "MIME-Version:", "multipart/alternative", "text/plain", "text/html"} {
		if !strings.Contains(s, want) {
			t.Errorf("MIME message missing %q", want)
		}
	}

	// Must NOT contain identifying user-agent.
	if strings.Contains(s, "Tackle") {
		t.Error("MIME message contains identifying string 'Tackle'")
	}
}

// --- Credential store ---

func TestCredentialStore(t *testing.T) {
	cs := newCredentialStore()

	cs.Store("host1", "user1", "pass1")
	cs.Store("host2", "user2", "pass2")

	c := cs.Load("host1")
	if c == nil || c.Username != "user1" || string(c.Password) != "pass1" {
		t.Errorf("host1 creds wrong: %+v", c)
	}

	cs.Clear("host1")
	if cs.Load("host1") != nil {
		t.Error("host1 should be cleared")
	}
	if cs.Load("host2") == nil {
		t.Error("host2 should still exist")
	}

	cs.ClearAll()
	if cs.Load("host2") != nil {
		t.Error("host2 should be cleared after ClearAll")
	}
}

// --- SMTP delivery via mock ---

func TestSMTPDeliveryViaMock(t *testing.T) {
	smtpAddr := startMockSMTP(t)
	relay := newSMTPRelay("ep", "", "", "0")

	// Parse host:port for the relay to connect to.
	host, port, _ := net.SplitHostPort(smtpAddr)
	var portNum int
	fmt.Sscanf(port, "%d", &portNum)

	cmd := &EmailSendCommand{
		ID: "deliver-1", Recipient: "r@example.com", Subject: "Test Delivery",
		BodyHTML: "<p>Hello</p>", BodyText: "Hello",
		SMTPHost: host, SMTPPort: portNum,
		EnvelopeSender: "sender@example.com", CampaignID: "camp-1",
	}

	result := relay.sendEmail(cmd)
	if !result.Success {
		t.Errorf("delivery failed: %s", result.ErrorMessage)
	}
	if result.SMTPCode != 250 {
		t.Errorf("SMTPCode = %d, want 250", result.SMTPCode)
	}
}

// --- Delivery to unreachable host ---

func TestSMTPConnectionRefused(t *testing.T) {
	relay := newSMTPRelay("ep", "", "", "0")
	cmd := &EmailSendCommand{
		ID: "fail-1", Recipient: "r@e.com", Subject: "S",
		BodyHTML: "<p>x</p>", SMTPHost: "127.0.0.1", SMTPPort: 1,
		EnvelopeSender: "s@e.com",
	}
	result := relay.sendEmail(cmd)
	if result.Success {
		t.Error("expected failure for unreachable host")
	}
	if result.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

// --- Health endpoint ---

func TestRelayHealthEndpoint(t *testing.T) {
	_, ts := newTestRelay(t, "")
	resp, err := http.Get(ts.URL + "/relay/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
}

// --- JSON serialization ---

func TestEmailSendCommandJSON(t *testing.T) {
	cmd := EmailSendCommand{
		ID: "j1", Recipient: "r@e.com", Subject: "S", BodyHTML: "<p>x</p>",
		SMTPHost: "smtp.e.com", SMTPPort: 587, RateLimit: 10, MaxConcurrent: 5,
	}
	b, _ := json.Marshal(cmd)
	var decoded EmailSendCommand
	json.Unmarshal(b, &decoded)
	if decoded.ID != "j1" || decoded.RateLimit != 10 || decoded.MaxConcurrent != 5 {
		t.Errorf("round-trip failed: %+v", decoded)
	}
}

func TestEmailSendResultJSON(t *testing.T) {
	r := EmailSendResult{ID: "r1", Success: true, SMTPCode: 250, DeliveredAt: time.Now().Format(time.RFC3339)}
	b, _ := json.Marshal(r)
	var decoded EmailSendResult
	json.Unmarshal(b, &decoded)
	if decoded.ID != "r1" || !decoded.Success {
		t.Errorf("round-trip failed: %+v", decoded)
	}
}

func TestSMTPLogEntryJSON(t *testing.T) {
	e := SMTPLogEntry{EndpointID: "ep", Status: "delivered", SMTPCode: 250}
	b, _ := json.Marshal(e)
	var decoded SMTPLogEntry
	json.Unmarshal(b, &decoded)
	if decoded.EndpointID != "ep" || decoded.Status != "delivered" {
		t.Errorf("round-trip failed: %+v", decoded)
	}
}

// --- Log delivery to framework ---

func TestSMTPLogToFramework(t *testing.T) {
	var receivedEntry SMTPLogEntry
	framework := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedEntry)
		w.WriteHeader(http.StatusOK)
	}))
	defer framework.Close()

	relay := newSMTPRelay("ep-1", framework.Listener.Addr().String(), "tok", "0")
	entry := SMTPLogEntry{
		EndpointID: "ep-1", CampaignID: "camp-1", CommandID: "cmd-1",
		Sender: "s@e.com", Recipient: "r@e.com", Status: "delivered",
		SMTPCode: 250, SMTPHost: "smtp.e.com", Timestamp: time.Now().Format(time.RFC3339),
	}
	err := relay.sendLogToFramework(entry)
	if err != nil {
		t.Fatalf("sendLogToFramework: %v", err)
	}
	if receivedEntry.CommandID != "cmd-1" {
		t.Errorf("framework got CommandID = %q, want cmd-1", receivedEntry.CommandID)
	}
}

// --- Authenticate function ---

func TestAuthenticate(t *testing.T) {
	relay := newSMTPRelay("ep", "", "my-token", "0")
	if !relay.authenticate("Bearer my-token") {
		t.Error("valid token rejected")
	}
	if relay.authenticate("Bearer wrong") {
		t.Error("wrong token accepted")
	}
	if relay.authenticate("") {
		t.Error("empty header accepted")
	}
	if relay.authenticate("Token my-token") {
		t.Error("wrong prefix accepted")
	}

	// No-auth relay.
	noAuth := newSMTPRelay("ep", "", "", "0")
	if !noAuth.authenticate("") {
		t.Error("no-auth relay should accept empty header")
	}
}
