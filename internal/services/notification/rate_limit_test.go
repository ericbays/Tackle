package notification

import (
	"testing"
	"time"
)

func TestRateLimitAllowsUnderLimit(t *testing.T) {
	svc := &NotificationService{
		rateLimit: make(map[string]*rateLimitEntry),
	}
	for i := 0; i < rateLimitPerUserPerHr; i++ {
		if svc.isRateLimited("user-1") {
			t.Fatalf("should not be rate limited at count %d", i+1)
		}
	}
}

func TestRateLimitBlocksOverLimit(t *testing.T) {
	svc := &NotificationService{
		rateLimit: make(map[string]*rateLimitEntry),
	}
	// Fill to limit.
	for i := 0; i < rateLimitPerUserPerHr; i++ {
		svc.isRateLimited("user-1")
	}
	// Next call should be blocked.
	if !svc.isRateLimited("user-1") {
		t.Fatal("should be rate limited after exceeding limit")
	}
}

func TestRateLimitResetsAfterWindow(t *testing.T) {
	svc := &NotificationService{
		rateLimit: make(map[string]*rateLimitEntry),
	}
	// Fill to limit.
	for i := 0; i < rateLimitPerUserPerHr; i++ {
		svc.isRateLimited("user-1")
	}

	// Simulate window reset by setting resetTime to past.
	svc.rateMu.Lock()
	svc.rateLimit["user-1"].resetTime = time.Now().Add(-1 * time.Second)
	svc.rateMu.Unlock()

	if svc.isRateLimited("user-1") {
		t.Fatal("should not be rate limited after window reset")
	}
}

func TestRateLimitIndependentPerUser(t *testing.T) {
	svc := &NotificationService{
		rateLimit: make(map[string]*rateLimitEntry),
	}
	// Fill user-1 to limit.
	for i := 0; i < rateLimitPerUserPerHr; i++ {
		svc.isRateLimited("user-1")
	}
	// user-2 should not be affected.
	if svc.isRateLimited("user-2") {
		t.Fatal("user-2 should not be rate limited")
	}
}
