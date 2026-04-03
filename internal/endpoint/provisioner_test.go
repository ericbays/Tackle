package endpoint

import (
	"context"
	"testing"
)

func TestSimpleDNSUpdater_CheckPropagation_NoResolver(t *testing.T) {
	// Test that CheckPropagation returns false when DNS cannot resolve.
	updater := &SimpleDNSUpdater{provider: nil}
	ctx := context.Background()

	// Use a domain that won't exist.
	propagated, err := updater.CheckPropagation(ctx, "nonexistent-domain-tackle-test.invalid", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if propagated {
		t.Error("expected false for nonexistent domain")
	}
}
