package audit

import (
	"testing"
	"time"
)

func newTestHMACSvc(t *testing.T) *HMACService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return NewHMACService(key)
}

func baseEntry() *LogEntry {
	id := "actor-1"
	return &LogEntry{
		ID:        "entry-1",
		Timestamp: time.Date(2026, 3, 11, 12, 0, 0, 0, time.UTC),
		Category:  CategoryUserActivity,
		Severity:  SeverityInfo,
		ActorType: ActorTypeUser,
		ActorID:   &id,
		Action:    "auth.login.success",
		Details:   map[string]any{"provider": "local"},
	}
}

func TestCompute_NonEmpty(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	checksum, err := svc.Compute(e)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	if checksum == "" {
		t.Fatal("expected non-empty checksum")
	}
	// Should be 64 hex characters (SHA-256 = 32 bytes = 64 hex).
	if len(checksum) != 64 {
		t.Errorf("checksum length = %d, want 64", len(checksum))
	}
}

func TestVerify_UnmodifiedEntry(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	checksum, err := svc.Compute(e)
	if err != nil {
		t.Fatalf("Compute error: %v", err)
	}
	e.Checksum = checksum
	if !svc.Verify(e) {
		t.Fatal("Verify returned false for unmodified entry")
	}
}

func TestVerify_TamperedAction(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	checksum, _ := svc.Compute(e)
	e.Checksum = checksum
	e.Action = "auth.login.failure" // tamper
	if svc.Verify(e) {
		t.Fatal("Verify returned true for tampered action")
	}
}

func TestVerify_TamperedChecksum(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	e.Checksum = "0000000000000000000000000000000000000000000000000000000000000000"
	if svc.Verify(e) {
		t.Fatal("Verify returned true for tampered checksum")
	}
}

func TestVerify_TamperedDetails(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	checksum, _ := svc.Compute(e)
	e.Checksum = checksum
	e.Details = map[string]any{"provider": "oauth"} // tamper
	if svc.Verify(e) {
		t.Fatal("Verify returned true for tampered details")
	}
}

func TestCompute_Deterministic(t *testing.T) {
	svc := newTestHMACSvc(t)
	e := baseEntry()
	c1, _ := svc.Compute(e)
	c2, _ := svc.Compute(e)
	if c1 != c2 {
		t.Fatal("Compute is not deterministic")
	}
}
