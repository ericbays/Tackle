package crypto

import (
	"bytes"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	svc, err := NewEncryptionService(testKey(t))
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}
	plaintext := []byte("hello, tackle!")
	ciphertext, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := svc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestEncryptUniqueNonce(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	plaintext := []byte("same plaintext")
	c1, _ := svc.Encrypt(plaintext)
	c2, _ := svc.Encrypt(plaintext)
	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions of the same plaintext must produce different ciphertexts")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	ciphertext, _ := svc.Encrypt([]byte("secret"))

	wrongKey := make([]byte, 32)
	wrongSvc, _ := NewEncryptionService(wrongKey)
	if _, err := wrongSvc.Decrypt(ciphertext); err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	ciphertext, _ := svc.Encrypt([]byte("secret"))
	ciphertext[len(ciphertext)-1] ^= 0xFF // flip bits in the tag
	if _, err := svc.Decrypt(ciphertext); err == nil {
		t.Fatal("expected error decrypting tampered ciphertext, got nil")
	}
}

func TestDecryptTruncatedCiphertext(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	if _, err := svc.Decrypt([]byte("short")); err == nil {
		t.Fatal("expected error decrypting truncated ciphertext, got nil")
	}
}

func TestNewEncryptionServiceKeyTooShort(t *testing.T) {
	if _, err := NewEncryptionService(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte key, got nil")
	}
}

func TestNewEncryptionServiceKeyTooLong(t *testing.T) {
	if _, err := NewEncryptionService(make([]byte, 64)); err == nil {
		t.Fatal("expected error for 64-byte key, got nil")
	}
}

func TestEncryptDecryptString(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	original := "encrypted string value"
	ct, err := svc.EncryptString(original)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}
	got, err := svc.DecryptString(ct)
	if err != nil {
		t.Fatalf("DecryptString: %v", err)
	}
	if got != original {
		t.Fatalf("got %q, want %q", got, original)
	}
}

func TestEncryptDecryptJSON(t *testing.T) {
	svc, _ := NewEncryptionService(testKey(t))
	type Config struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	original := Config{Host: "smtp.example.com", Port: 587}
	ct, err := svc.EncryptJSON(original)
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	var got Config
	if err := svc.DecryptJSON(ct, &got); err != nil {
		t.Fatalf("DecryptJSON: %v", err)
	}
	if got != original {
		t.Fatalf("got %+v, want %+v", got, original)
	}
}
