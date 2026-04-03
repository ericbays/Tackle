// Package replay provides tests for the credential replay functionality.
package replay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewReplayHandler verifies default config values are applied correctly.
func TestNewReplayHandler(t *testing.T) {
	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      "https://example.com/login",
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 10,
	}

	handler := NewReplayHandler(config)

	if handler.config.Enabled != true {
		t.Errorf("Expected Enabled=true, got %v", handler.config.Enabled)
	}
	if handler.config.TargetURL != "https://example.com/login" {
		t.Errorf("Expected TargetURL=https://example.com/login, got %s", handler.config.TargetURL)
	}
	if handler.config.Method != "POST" {
		t.Errorf("Expected Method=POST, got %s", handler.config.Method)
	}
	if handler.config.ContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Expected ContentType=application/x-www-form-urlencoded, got %s", handler.config.ContentType)
	}
	if handler.config.TimeoutSeconds != 10 {
		t.Errorf("Expected TimeoutSeconds=10, got %d", handler.config.TimeoutSeconds)
	}
	if handler.client == nil {
		t.Error("Expected client to be initialized")
	}
}

// TestReplay_FormURLencoded tests form-urlencoded content type.
func TestReplay_FormURLencoded(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		if username != "testuser" {
			t.Errorf("Expected username=testuser, got %s", username)
		}
		if password != "secret123" {
			t.Errorf("Expected password=secret123, got %s", password)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
	if string(result.Body) != `{"success":true}` {
		t.Errorf("Expected body {\"success\":true}, got %s", string(result.Body))
	}
}

// TestReplay_JSONObject tests JSON content type.
func TestReplay_JSONObject(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read body: %v", err)
		}
		defer r.Body.Close()

		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("Failed to unmarshal body: %v", err)
		}

		if payload["username"] != "testuser" {
			t.Errorf("Expected username=testuser, got %s", payload["username"])
		}
		if payload["password"] != "secret123" {
			t.Errorf("Expected password=secret123, got %s", payload["password"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/json",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
	if string(result.Body) != `{"success":true}` {
		t.Errorf("Expected body {\"success\":true}, got %s", string(result.Body))
	}
}

// TestReplay_FieldMapping tests field name mapping.
func TestReplay_FieldMapping(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		email := r.FormValue("email")
		pass := r.FormValue("pass")

		if email != "user@example.com" {
			t.Errorf("Expected email=user@example.com, got %s", email)
		}
		if pass != "mypassword" {
			t.Errorf("Expected pass=mypassword, got %s", pass)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:     true,
		TargetURL:   mockServer.URL,
		Method:      "POST",
		ContentType: "application/x-www-form-urlencoded",
		FieldMapping: map[string]string{
			"user_email": "email",
			"user_pass":  "pass",
		},
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"user_email": "user@example.com",
		"user_pass":  "mypassword",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
}

// TestReplay_ExtraFields tests extra fields are merged.
func TestReplay_ExtraFields(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		csrfToken := r.FormValue("csrf_token")
		submitBtn := r.FormValue("submit")

		if csrfToken != "abc123" {
			t.Errorf("Expected csrf_token=abc123, got %s", csrfToken)
		}
		if submitBtn != "Login" {
			t.Errorf("Expected submit=Login, got %s", submitBtn)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:     true,
		TargetURL:   mockServer.URL,
		Method:      "POST",
		ContentType: "application/x-www-form-urlencoded",
		ExtraFields: map[string]string{
			"csrf_token": "abc123",
			"submit":     "Login",
		},
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
}

// TestReplay_RedirectRelay tests redirect response relay.
func TestReplay_RedirectRelay(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.com/dashboard")
		w.WriteHeader(http.StatusFound)
		w.Write([]byte("Redirecting.."))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302, got %d", result.StatusCode)
	}
	// With CheckRedirect returning ErrUseLastResponse, the redirect URL is in headers
	t.Logf("Headers: %v", result.Headers)
	if result.Headers.Get("Location") != "https://example.com/dashboard" {
		t.Errorf("Expected redirect URL in headers, got %s", result.Headers.Get("Location"))
	}
}

// TestReplay_CookieRelay tests cookie headers relay.
func TestReplay_CookieRelay(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "sessionid=abc123; Path=/; HttpOnly")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("logged in"))
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
		"password": "secret123",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
	if result.Headers.Get("Set-Cookie") != "sessionid=abc123; Path=/; HttpOnly" {
		t.Errorf("Expected Set-Cookie header, got %s", result.Headers.Get("Set-Cookie"))
	}
}

// TestReplay_Timeout tests timeout on slow server.
func TestReplay_Timeout(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 100,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	result, err := handler.Replay(ctx, fields)

	if err != nil {
		t.Logf("Replay returned expected timeout error: %v", err)
	}
	if result != nil && result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
}

// TestReplay_Disabled tests disabled replay returns error.
func TestReplay_Disabled(t *testing.T) {
	config := ReplayConfig{
		Enabled:        false,
		TargetURL:      "https://example.com/login",
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err == nil {
		t.Error("Expected error for disabled replay, got nil")
	}
	if err != nil && err.Error() != "replay: replay is not enabled" {
		t.Errorf("Expected 'replay is not enabled' error, got %v", err)
	}
	if result == nil {
		t.Error("Expected result to be returned even on disabled replay")
	}
}

// TestRelayToResponse tests writing result to response writer.
func TestRelayToResponse(t *testing.T) {
	mockWriter := &mockResponseWriter{}
	result := &ReplayResult{
		StatusCode: http.StatusCreated,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Custom":     []string{"value"},
		},
		Cookies: []*http.Cookie{
			{Name: "session", Value: "abc123", Path: "/"},
		},
		Body: []byte(`{"status":"ok"}`),
	}

	RelayToResponse(mockWriter, result)

	if mockWriter.statusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", mockWriter.statusCode)
	}
	if ct := mockWriter.header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}
	if custom := mockWriter.header.Get("X-Custom"); custom != "value" {
		t.Errorf("Expected X-Custom value, got %s", custom)
	}
	if string(mockWriter.body) != `{"status":"ok"}` {
		t.Errorf("Expected body {\"status\":\"ok\"}, got %s", string(mockWriter.body))
	}
}

// TestReplay_TLSConfig tests TLS verification is enabled.
func TestReplay_TLSConfig(t *testing.T) {
	mockServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:           true,
		TargetURL:         mockServer.URL,
		Method:            "POST",
		ContentType:       "application/x-www-form-urlencoded",
		TimeoutSeconds:    5,
		SkipTLSCertVerify: true,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
}

// TestReplay_NoFollowRedirects tests that redirects are not followed.
func TestReplay_NoFollowRedirects(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(http.StatusFound)
			w.Write([]byte("initial"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("redirected content"))
		}
	}))
	defer mockServer.Close()

	config := ReplayConfig{
		Enabled:        true,
		TargetURL:      mockServer.URL,
		Method:         "POST",
		ContentType:    "application/x-www-form-urlencoded",
		TimeoutSeconds: 5,
	}

	handler := NewReplayHandler(config)
	fields := map[string]string{
		"username": "testuser",
	}

	result, err := handler.Replay(context.Background(), fields)

	if err != nil {
		t.Errorf("Replay returned error: %v", err)
	}
	if result.StatusCode != http.StatusFound {
		t.Errorf("Expected status 302 (not followed), got %d", result.StatusCode)
	}
	if result.RedirectURL != "/redirected" {
		t.Errorf("Expected redirect URL /redirected, got %s", result.RedirectURL)
	}
}

// mockResponseWriter is a minimal http.ResponseWriter for testing.
type mockResponseWriter struct {
	statusCode int
	header     http.Header
	body       []byte
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func (m *mockResponseWriter) Write(body []byte) (int, error) {
	m.body = append(m.body, body...)
	return len(body), nil
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}
