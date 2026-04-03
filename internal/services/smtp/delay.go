package smtp

import (
	"fmt"
	"time"
)

// GenerateDelay returns a cryptographically random delay duration between minMs and maxMs.
// Returns 0 if both are 0. Returns minMs if minMs == maxMs.
func GenerateDelay(minMs, maxMs int) (time.Duration, error) {
	if minMs < 0 {
		minMs = 0
	}
	if maxMs < minMs {
		maxMs = minMs
	}
	if minMs == 0 && maxMs == 0 {
		return 0, nil
	}
	if minMs == maxMs {
		return time.Duration(minMs) * time.Millisecond, nil
	}
	rangeSize := maxMs - minMs
	offset, err := cryptoRandInt(rangeSize + 1)
	if err != nil {
		return 0, fmt.Errorf("generate delay: %w", err)
	}
	return time.Duration(minMs+offset) * time.Millisecond, nil
}

// GenerateBatchDelay returns the batch pause duration when messageIndex is a multiple of batchSize.
// Returns 0 for all other messages or if batching is not configured.
func GenerateBatchDelay(batchSize, batchPauseSeconds, messageIndex int) time.Duration {
	if batchSize <= 0 || batchPauseSeconds <= 0 {
		return 0
	}
	// Index 0 is the first message — no pause before it.
	if messageIndex == 0 {
		return 0
	}
	if messageIndex%batchSize == 0 {
		return time.Duration(batchPauseSeconds) * time.Second
	}
	return 0
}
