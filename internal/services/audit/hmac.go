package audit

import (
	"encoding/json"
	"fmt"

	"tackle/internal/crypto"
)

// HMACService computes and verifies HMAC-SHA256 checksums for audit log entries.
// Construct via NewHMACService with a key derived using crypto.PurposeHMACAudit.
type HMACService struct {
	inner *crypto.HMACService
}

// NewHMACService creates an HMACService bound to key.
// key must be a subkey derived via crypto.DeriveSubkey(masterKey, crypto.PurposeHMACAudit).
func NewHMACService(key []byte) *HMACService {
	return &HMACService{inner: crypto.NewHMACService(key)}
}

// Compute derives an HMAC-SHA256 checksum over the entry's core fields:
// timestamp, category, action, actor_id, resource_id, details JSON,
// and the previous entry's checksum (forming a hash chain).
// Returns a lowercase hex string.
func (h *HMACService) Compute(e *LogEntry) (string, error) {
	detailsJSON, err := marshalDetails(e.Details)
	if err != nil {
		return "", fmt.Errorf("audit hmac: marshal details: %w", err)
	}
	actorID := ""
	if e.ActorID != nil {
		actorID = *e.ActorID
	}
	resourceID := ""
	if e.ResourceID != nil {
		resourceID = *e.ResourceID
	}
	ts := e.Timestamp.UTC().Format("2006-01-02T15:04:05.999999Z")
	checksum := h.inner.ComputeChecksum(ts, string(e.Category), e.Action, actorID, resourceID, detailsJSON, e.PreviousChecksum)
	return checksum, nil
}

// Verify recomputes the HMAC for e and compares it to e.Checksum.
// Returns true if the entry has not been tampered with.
func (h *HMACService) Verify(e *LogEntry) bool {
	expected, err := h.Compute(e)
	if err != nil {
		return false
	}
	return e.Checksum == expected
}

// marshalDetails serialises details to a canonical JSON string, or returns
// "null" for a nil map.
func marshalDetails(details map[string]any) (string, error) {
	if details == nil {
		return "null", nil
	}
	b, err := json.Marshal(details)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
