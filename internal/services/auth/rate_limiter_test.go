package auth

import (
	"strings"
	"testing"
)

func TestRateLimiter_IP_BlocksAfterThreshold(t *testing.T) {
	rl := NewRateLimiter()
	key := "ip:1.2.3.4"

	var locked bool
	for i := 0; i < 10; i++ {
		_, _, locked = rl.RecordFailure(key)
	}
	if !locked {
		t.Error("expected IP to be locked after 10 failures")
	}
	isLocked, _ := rl.IsLocked(key)
	if !isLocked {
		t.Error("IsLocked should return true after threshold")
	}
}

func TestRateLimiter_Account_BlocksAfterThreshold(t *testing.T) {
	rl := NewRateLimiter()
	key := "user:alice"

	var locked bool
	for i := 0; i < 5; i++ {
		_, _, locked = rl.RecordFailure(key)
	}
	if !locked {
		t.Error("expected account to be locked after 5 failures")
	}
}

func TestRateLimiter_Reset_ClearsLock(t *testing.T) {
	rl := NewRateLimiter()
	key := "user:bob"

	for i := 0; i < 5; i++ {
		rl.RecordFailure(key)
	}
	isLocked, _ := rl.IsLocked(key)
	if !isLocked {
		t.Fatal("expected locked before reset")
	}

	rl.Reset(key)
	isLocked, _ = rl.IsLocked(key)
	if isLocked {
		t.Error("expected not locked after reset")
	}
}

func TestRateLimiter_NotLockedBeforeThreshold(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 4; i++ {
		_, _, locked := rl.RecordFailure("user:carol")
		if locked {
			t.Errorf("should not be locked after %d failures", i+1)
		}
	}
}

func TestRateLimiter_IPPrefix(t *testing.T) {
	rl := NewRateLimiter()
	// IP threshold is 10, account is 5. Verify IP key uses higher threshold.
	key := "ip:10.0.0.1"
	var locked bool
	for i := 0; i < 9; i++ {
		_, _, locked = rl.RecordFailure(key)
	}
	if locked {
		t.Error("IP should not be locked after 9 failures (threshold is 10)")
	}
	_, _, locked = rl.RecordFailure(key)
	if !locked {
		t.Error("IP should be locked after 10 failures")
	}
}

func TestRateLimiter_CountIncrements(t *testing.T) {
	rl := NewRateLimiter()
	key := "user:dave"
	for i := 1; i <= 3; i++ {
		count, _, _ := rl.RecordFailure(key)
		if count != i {
			t.Errorf("attempt %d: count = %d, want %d", i, count, i)
		}
	}
}

func TestRateLimiter_KeyIsolation(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 5; i++ {
		rl.RecordFailure("user:alice")
	}
	isLocked, _ := rl.IsLocked("user:bob")
	if isLocked {
		t.Error("alice's lockout should not affect bob")
	}
	_ = strings.Contains("", "") // suppress import
}
