package auth

import (
	"testing"
	"time"
)

func TestTokenBlacklist_RevokeAndCheck(t *testing.T) {
	bl := NewTokenBlacklist()
	jti := "test-jti-1234"

	if bl.IsRevoked(jti) {
		t.Error("expected not revoked before Revoke call")
	}

	bl.Revoke(jti, time.Now().Add(15*time.Minute))

	if !bl.IsRevoked(jti) {
		t.Error("expected revoked after Revoke call")
	}
}

func TestTokenBlacklist_ExpiredEntry_NotRevoked(t *testing.T) {
	bl := NewTokenBlacklist()
	jti := "expired-jti"

	// Revoke with a past expiry.
	bl.Revoke(jti, time.Now().Add(-1*time.Second))

	if bl.IsRevoked(jti) {
		t.Error("expected expired entry to not be considered revoked")
	}
}

func TestTokenBlacklist_ExpiredEntry_Cleanup(t *testing.T) {
	bl := NewTokenBlacklist()
	bl.Revoke("jti-1", time.Now().Add(-time.Second))
	bl.Revoke("jti-2", time.Now().Add(time.Minute))

	// Trigger cleanup for jti-1.
	bl.IsRevoked("jti-1")

	bl.mu.RLock()
	defer bl.mu.RUnlock()
	if _, ok := bl.entries["jti-1"]; ok {
		t.Error("expected expired entry to be removed from map")
	}
	if _, ok := bl.entries["jti-2"]; !ok {
		t.Error("expected non-expired entry to remain in map")
	}
}

func TestTokenBlacklist_UnknownJTI(t *testing.T) {
	bl := NewTokenBlacklist()
	if bl.IsRevoked("unknown-jti") {
		t.Error("expected unknown JTI to return false")
	}
}
