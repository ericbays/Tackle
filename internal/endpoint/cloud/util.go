package cloud

import (
	"crypto/rand"
	"encoding/hex"
)

// generateShortID returns a random 8-character hex string for naming cloud resources.
func generateShortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
