// Package crypto provides centralized cryptographic services for the Tackle platform.
package crypto

import (
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// Purpose constants for HKDF subkey derivation. Each purpose produces an
// independent 32-byte subkey so that a single master key is never reused
// across different cryptographic operations.
const (
	// PurposeDBEncryption is used for AES-256-GCM encryption of database columns.
	PurposeDBEncryption = "tackle/db-encryption"
	// PurposeHMACAudit is used for HMAC-SHA256 checksums on audit log entries.
	PurposeHMACAudit = "tackle/hmac-audit"
	// PurposeJWTSigning is used for JWT access token and refresh token signing.
	PurposeJWTSigning = "tackle/jwt-signing"
	// PurposeProviderCredentials is used for AES-256-GCM encryption of domain provider credentials.
	PurposeProviderCredentials = "tackle/provider-credentials"
	// PurposeTrackingToken is used for HMAC-SHA256 deterministic tracking token generation.
	PurposeTrackingToken = "tackle/tracking-token"
)

// DeriveSubkey derives a 32-byte subkey from masterKey using HKDF-SHA256.
// purpose is the HKDF info string that domain-separates the derived keys.
// The same masterKey and purpose always produce the same subkey.
func DeriveSubkey(masterKey []byte, purpose string) ([]byte, error) {
	if len(masterKey) == 0 {
		return nil, fmt.Errorf("derive subkey: master key must not be empty")
	}
	r := hkdf.New(sha256.New, masterKey, nil, []byte(purpose))
	subkey := make([]byte, 32)
	if _, err := io.ReadFull(r, subkey); err != nil {
		return nil, fmt.Errorf("derive subkey: hkdf read: %w", err)
	}
	return subkey, nil
}
