package tracking

import (
	"encoding/base64"
	"strings"
	"testing"

	"tackle/internal/crypto"
)

// newTestService creates a TokenService with a deterministic key for testing.
func newTestService(t *testing.T) *TokenService {
	t.Helper()
	key := []byte("test-key-32-bytes-for-unit-test!")
	return NewTokenService(crypto.NewHMACService(key))
}

func TestGenerateToken_ValidFormat(t *testing.T) {
	ts := newTestService(t)

	token, err := ts.GenerateToken("campaign-1", "target-1")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Must contain exactly one separator.
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		t.Fatalf("token should have 2 parts separated by '.', got %d: %q", len(parts), token)
	}

	// Random component should be valid base64url (22 chars for 16 bytes).
	randomBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("random component is not valid base64url: %v", err)
	}
	if len(randomBytes) != tokenRandomBytes {
		t.Errorf("random component decoded to %d bytes, want %d", len(randomBytes), tokenRandomBytes)
	}

	// Signature should be hex-encoded (16 chars for 8 bytes).
	if len(parts[1]) != tokenHMACBytes*2 {
		t.Errorf("signature length = %d, want %d", len(parts[1]), tokenHMACBytes*2)
	}
}

func TestGenerateToken_Unique(t *testing.T) {
	ts := newTestService(t)
	seen := make(map[string]bool, 1000)

	for i := 0; i < 1000; i++ {
		token, err := ts.GenerateToken("campaign-1", "target-1")
		if err != nil {
			t.Fatalf("iteration %d: GenerateToken() error = %v", i, err)
		}
		if seen[token] {
			t.Fatalf("duplicate token at iteration %d: %s", i, token)
		}
		seen[token] = true
	}
}

func TestGenerateToken_URLSafe(t *testing.T) {
	ts := newTestService(t)

	for i := 0; i < 100; i++ {
		token, err := ts.GenerateToken("campaign-1", "target-1")
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if strings.ContainsAny(token, "+/= \t\n") {
			t.Errorf("token contains non-URL-safe characters: %q", token)
		}
	}
}

func TestGenerateToken_Opaque(t *testing.T) {
	ts := newTestService(t)

	campaignID := "abc12345-dead-beef-cafe-000000000001"
	targetID := "abc12345-dead-beef-cafe-000000000002"

	token, err := ts.GenerateToken(campaignID, targetID)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if strings.Contains(token, campaignID) {
		t.Errorf("token should not contain campaign ID")
	}
	if strings.Contains(token, targetID) {
		t.Errorf("token should not contain target ID")
	}
}

func TestGenerateToken_CrossCampaignIsolation(t *testing.T) {
	ts := newTestService(t)

	token1, _ := ts.GenerateToken("campaign-1", "target-1")
	token2, _ := ts.GenerateToken("campaign-2", "target-1")

	if token1 == token2 {
		t.Error("same target in different campaigns should produce different tokens")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	ts := newTestService(t)

	token, err := ts.GenerateToken("campaign-1", "target-1")
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if !ts.ValidateToken(token, "campaign-1", "target-1") {
		t.Error("ValidateToken() should return true for a valid token")
	}
}

func TestValidateToken_WrongCampaign(t *testing.T) {
	ts := newTestService(t)

	token, _ := ts.GenerateToken("campaign-1", "target-1")

	if ts.ValidateToken(token, "campaign-2", "target-1") {
		t.Error("ValidateToken() should return false for wrong campaign")
	}
}

func TestValidateToken_WrongTarget(t *testing.T) {
	ts := newTestService(t)

	token, _ := ts.GenerateToken("campaign-1", "target-1")

	if ts.ValidateToken(token, "campaign-1", "target-2") {
		t.Error("ValidateToken() should return false for wrong target")
	}
}

func TestValidateToken_Tampered(t *testing.T) {
	ts := newTestService(t)

	token, _ := ts.GenerateToken("campaign-1", "target-1")

	// Flip a character in the random component.
	tampered := []byte(token)
	if tampered[0] == 'a' {
		tampered[0] = 'b'
	} else {
		tampered[0] = 'a'
	}

	if ts.ValidateToken(string(tampered), "campaign-1", "target-1") {
		t.Error("ValidateToken() should return false for tampered token")
	}
}

func TestValidateToken_EmptyToken(t *testing.T) {
	ts := newTestService(t)

	if ts.ValidateToken("", "campaign-1", "target-1") {
		t.Error("ValidateToken() should return false for empty token")
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	ts := newTestService(t)

	malformed := []string{
		"no-separator",
		"too.many.separators",
		".leading-dot",
		"trailing-dot.",
	}

	for _, tok := range malformed {
		if ts.ValidateToken(tok, "campaign-1", "target-1") {
			t.Errorf("ValidateToken(%q) should return false", tok)
		}
	}
}

func TestParseToken_Valid(t *testing.T) {
	random, sig, err := ParseToken("randompart.signaturepart")
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}
	if random != "randompart" {
		t.Errorf("random = %q, want %q", random, "randompart")
	}
	if sig != "signaturepart" {
		t.Errorf("signature = %q, want %q", sig, "signaturepart")
	}
}

func TestParseToken_Empty(t *testing.T) {
	_, _, err := ParseToken("")
	if err == nil {
		t.Error("ParseToken(\"\") should return error")
	}
}

func TestParseToken_NoSeparator(t *testing.T) {
	_, _, err := ParseToken("noseparator")
	if err == nil {
		t.Error("ParseToken(\"noseparator\") should return error")
	}
}

func TestParseToken_TooManySeparators(t *testing.T) {
	_, _, err := ParseToken("a.b.c")
	if err == nil {
		t.Error("ParseToken(\"a.b.c\") should return error")
	}
}

func TestEmbedTokenInURL_Basic(t *testing.T) {
	result, err := EmbedTokenInURL("https://example.com/login", "tok123.sig456", "_t")
	if err != nil {
		t.Fatalf("EmbedTokenInURL() error = %v", err)
	}
	expected := "https://example.com/login?_t=tok123.sig456"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestEmbedTokenInURL_ExistingParams(t *testing.T) {
	result, err := EmbedTokenInURL("https://example.com/login?ref=email", "tok123.sig456", "_t")
	if err != nil {
		t.Fatalf("EmbedTokenInURL() error = %v", err)
	}
	if !strings.Contains(result, "ref=email") {
		t.Error("existing param should be preserved")
	}
	if !strings.Contains(result, "_t=tok123.sig456") {
		t.Error("token param should be added")
	}
}

func TestEmbedTokenInURL_DefaultParam(t *testing.T) {
	result, err := EmbedTokenInURL("https://example.com/login", "tok123.sig456", "")
	if err != nil {
		t.Fatalf("EmbedTokenInURL() error = %v", err)
	}
	if !strings.Contains(result, "_t=tok123.sig456") {
		t.Errorf("default param name should be _t, got %q", result)
	}
}

func TestEmbedTokenInURL_WithFragment(t *testing.T) {
	result, err := EmbedTokenInURL("https://example.com/login#section", "tok123.sig456", "_t")
	if err != nil {
		t.Fatalf("EmbedTokenInURL() error = %v", err)
	}
	if !strings.Contains(result, "_t=tok123.sig456") {
		t.Errorf("token should be in query, got %q", result)
	}
	if !strings.Contains(result, "#section") {
		t.Errorf("fragment should be preserved, got %q", result)
	}
}

func TestEmbedTokenInURL_InvalidURL(t *testing.T) {
	_, err := EmbedTokenInURL("://invalid", "tok123.sig456", "_t")
	if err == nil {
		t.Error("EmbedTokenInURL() should return error for invalid URL")
	}
}

func TestRandomBytes(t *testing.T) {
	ts := newTestService(t)
	token, _ := ts.GenerateToken("c1", "t1")

	rb, err := ts.RandomBytes(token)
	if err != nil {
		t.Fatalf("RandomBytes() error = %v", err)
	}
	if len(rb) != tokenRandomBytes {
		t.Errorf("RandomBytes() returned %d bytes, want %d", len(rb), tokenRandomBytes)
	}
}

func TestSignatureBytes(t *testing.T) {
	ts := newTestService(t)
	token, _ := ts.GenerateToken("c1", "t1")

	sb, err := ts.SignatureBytes(token)
	if err != nil {
		t.Fatalf("SignatureBytes() error = %v", err)
	}
	if len(sb) != tokenHMACBytes {
		t.Errorf("SignatureBytes() returned %d bytes, want %d", len(sb), tokenHMACBytes)
	}
}

func BenchmarkGenerateToken(b *testing.B) {
	key := []byte("test-key-32-bytes-for-unit-test!")
	ts := NewTokenService(crypto.NewHMACService(key))
	for i := 0; i < b.N; i++ {
		_, _ = ts.GenerateToken("campaign-1", "target-1")
	}
}
