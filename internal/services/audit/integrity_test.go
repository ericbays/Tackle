package audit

import "testing"

func TestVerify_ValidChecksum(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	checksum, err := svc.Compute(e)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	e.Checksum = checksum
	if !svc.Verify(e) {
		t.Fatal("Verify returned false for valid checksum")
	}
}

func TestVerify_InvalidChecksum(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	// Set a known-bad checksum.
	e.Checksum = "aabbccdd"
	if svc.Verify(e) {
		t.Fatal("Verify returned true for tampered checksum field")
	}
}
