package credentials

import (
	"fmt"

	"tackle/internal/crypto"
)

// PurposeProviderCredentials is the HKDF purpose string for provider credential encryption.
const PurposeProviderCredentials = "tackle/provider-credentials"

// EncryptionService wraps the crypto.EncryptionService to encrypt/decrypt provider credentials.
type EncryptionService struct {
	enc *crypto.EncryptionService
}

// NewEncryptionService creates a credential EncryptionService from the platform master key.
func NewEncryptionService(masterKey []byte) (*EncryptionService, error) {
	enc, err := crypto.NewEncryptionServiceForPurpose(masterKey, PurposeProviderCredentials)
	if err != nil {
		return nil, fmt.Errorf("provider credentials: %w", err)
	}
	return &EncryptionService{enc: enc}, nil
}

// Encrypt marshals creds as JSON and encrypts the result using AES-256-GCM.
// creds must be one of the typed credential structs (raw JSON marshaling is used for storage,
// not the masked MarshalJSON — callers must pass the struct directly, not via interface).
func (s *EncryptionService) Encrypt(creds any) ([]byte, error) {
	b, err := marshalRaw(creds)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: marshal: %w", err)
	}
	ct, err := s.enc.Encrypt(b)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}
	return ct, nil
}

// Decrypt decrypts ciphertext and unmarshals the result into target.
// target must be a pointer to one of the typed credential structs.
func (s *EncryptionService) Decrypt(ciphertext []byte, target any) error {
	b, err := s.enc.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt credentials: %w", err)
	}
	if err := unmarshalRaw(b, target); err != nil {
		return fmt.Errorf("decrypt credentials: unmarshal: %w", err)
	}
	return nil
}
