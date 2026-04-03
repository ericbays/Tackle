package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HMACService computes and verifies HMAC-SHA256 checksums.
// It is used to generate and validate integrity checksums on audit log entries.
// Always construct with a subkey derived via DeriveSubkey — never the raw master key.
type HMACService struct {
	key []byte
}

// NewHMACService creates an HMACService bound to key.
// key should be a subkey derived via DeriveSubkey(masterKey, PurposeHMACAudit).
func NewHMACService(key []byte) *HMACService {
	k := make([]byte, len(key))
	copy(k, key)
	return &HMACService{key: k}
}

// ComputeChecksum concatenates fields with a NUL byte separator, computes
// HMAC-SHA256, and returns a lowercase hex string.
func (h *HMACService) ComputeChecksum(fields ...string) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(strings.Join(fields, "\x00")))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyChecksum returns true if checksum matches the HMAC of fields.
// Uses constant-time comparison to prevent timing attacks.
func (h *HMACService) VerifyChecksum(checksum string, fields ...string) bool {
	expected := h.ComputeChecksum(fields...)
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	gotBytes, err := hex.DecodeString(checksum)
	if err != nil {
		return false
	}
	return hmac.Equal(expectedBytes, gotBytes)
}
