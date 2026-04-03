package crypto

import (
	"testing"
)

func hmacTestKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 50)
	}
	return key
}

func TestHMACDeterministic(t *testing.T) {
	svc := NewHMACService(hmacTestKey())
	c1 := svc.ComputeChecksum("2026-01-01", "auth", "login", "user-123", "resource-456", `{"ip":"10.0.0.1"}`)
	c2 := svc.ComputeChecksum("2026-01-01", "auth", "login", "user-123", "resource-456", `{"ip":"10.0.0.1"}`)
	if c1 != c2 {
		t.Fatalf("ComputeChecksum must be deterministic: got %q and %q", c1, c2)
	}
}

func TestHMACVerifyValid(t *testing.T) {
	svc := NewHMACService(hmacTestKey())
	fields := []string{"ts", "cat", "act", "uid", "rid", "{}"}
	checksum := svc.ComputeChecksum(fields...)
	if !svc.VerifyChecksum(checksum, fields...) {
		t.Fatal("VerifyChecksum returned false for a valid checksum")
	}
}

func TestHMACVerifyTamperedData(t *testing.T) {
	svc := NewHMACService(hmacTestKey())
	checksum := svc.ComputeChecksum("field1", "field2")
	if svc.VerifyChecksum(checksum, "field1", "TAMPERED") {
		t.Fatal("VerifyChecksum returned true for tampered data")
	}
}

func TestHMACVerifyWrongKey(t *testing.T) {
	svc := NewHMACService(hmacTestKey())
	checksum := svc.ComputeChecksum("field1", "field2")

	wrongKey := make([]byte, 32)
	wrongSvc := NewHMACService(wrongKey)
	if wrongSvc.VerifyChecksum(checksum, "field1", "field2") {
		t.Fatal("VerifyChecksum returned true with wrong key")
	}
}
