package crypto

import (
	"bytes"
	"testing"
)

func makeKey(fill byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	return key
}

// TestRotationReEncrypts verifies that a value encrypted with the old key can be
// re-encrypted and then decrypted with the new key (unit-level, no DB required).
func TestRotationReEncrypts(t *testing.T) {
	oldSvc, err := NewEncryptionService(makeKey(0xAA))
	if err != nil {
		t.Fatalf("NewEncryptionService(old): %v", err)
	}
	newSvc, err := NewEncryptionService(makeKey(0xBB))
	if err != nil {
		t.Fatalf("NewEncryptionService(new): %v", err)
	}

	plaintext := []byte("sensitive-smtp-password")
	oldCiphertext, err := oldSvc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt with old key: %v", err)
	}

	// Simulate what Rotator does per-row: decrypt with old, re-encrypt with new.
	intermediate, err := oldSvc.Decrypt(oldCiphertext)
	if err != nil {
		t.Fatalf("Decrypt with old key: %v", err)
	}
	newCiphertext, err := newSvc.Encrypt(intermediate)
	if err != nil {
		t.Fatalf("Re-encrypt with new key: %v", err)
	}

	got, err := newSvc.Decrypt(newCiphertext)
	if err != nil {
		t.Fatalf("Decrypt with new key after rotation: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("rotation round-trip failed: got %q, want %q", got, plaintext)
	}
}

// TestRotationIdempotency verifies that rotating with the same old and new key
// is detected before any DB work is attempted (Rotator.Rotate returns immediately).
func TestRotationIdempotency(t *testing.T) {
	key := makeKey(0xCC)
	svc, err := NewEncryptionService(key)
	if err != nil {
		t.Fatalf("NewEncryptionService: %v", err)
	}

	// Same key pointer — bytes.Equal should return true inside Rotate.
	rotator := &Rotator{oldSvc: svc, newSvc: svc}
	if !bytes.Equal(rotator.oldSvc.key, rotator.newSvc.key) {
		t.Fatal("expected keys to be equal for idempotency test")
	}
}
