package audit

import "testing"

func TestSanitizeDetails_SecretFieldsRedacted(t *testing.T) {
	cases := []string{
		"password", "Password", "PASSWORD",
		"token", "access_token", "refresh_token",
		"secret", "client_secret",
		"api_key", "private_key",
	}
	for _, k := range cases {
		input := map[string]any{k: "supersecret"}
		out := SanitizeDetails(input)
		if out[k] != "[REDACTED]" {
			t.Errorf("field %q: got %q, want [REDACTED]", k, out[k])
		}
	}
}

func TestSanitizeDetails_NonSecretPreserved(t *testing.T) {
	input := map[string]any{
		"username": "alice",
		"action":   "login",
		"count":    42,
	}
	out := SanitizeDetails(input)
	if out["username"] != "alice" {
		t.Errorf("username: got %v, want alice", out["username"])
	}
	if out["action"] != "login" {
		t.Errorf("action: got %v, want login", out["action"])
	}
	if out["count"] != 42 {
		t.Errorf("count: got %v, want 42", out["count"])
	}
}

func TestSanitizeDetails_InjectionEncoded(t *testing.T) {
	input := map[string]any{"msg": "line1\nline2\rend\t\x00null"}
	out := SanitizeDetails(input)
	got, ok := out["msg"].(string)
	if !ok {
		t.Fatal("msg is not a string")
	}
	if got != `line1\nline2\rend\t\0null` {
		t.Errorf("injection not encoded: got %q", got)
	}
}

func TestSanitizeDetails_Nil(t *testing.T) {
	if SanitizeDetails(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestSanitizeDetails_NestedMap(t *testing.T) {
	inner := map[string]any{"password": "secret", "name": "bob"}
	input := map[string]any{"user": inner}
	out := SanitizeDetails(input)
	nested, ok := out["user"].(map[string]any)
	if !ok {
		t.Fatal("nested value is not a map")
	}
	if nested["password"] != "[REDACTED]" {
		t.Errorf("nested password not redacted: got %v", nested["password"])
	}
	if nested["name"] != "bob" {
		t.Errorf("nested name: got %v, want bob", nested["name"])
	}
}
