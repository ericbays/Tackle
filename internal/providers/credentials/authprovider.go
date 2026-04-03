package credentials

import (
	"fmt"

	"tackle/internal/crypto"
)

// PurposeAuthProviderCredentials is the HKDF purpose string for auth provider credential encryption.
const PurposeAuthProviderCredentials = "tackle/auth-provider-credentials"

// AuthProviderEncryptionService encrypts and decrypts auth provider configuration blobs.
// The configuration is stored as AES-256-GCM encrypted JSON in the auth_providers table.
type AuthProviderEncryptionService struct {
	enc *crypto.EncryptionService
}

// NewAuthProviderEncryptionService creates an AuthProviderEncryptionService from the platform master key.
func NewAuthProviderEncryptionService(masterKey []byte) (*AuthProviderEncryptionService, error) {
	enc, err := crypto.NewEncryptionServiceForPurpose(masterKey, PurposeAuthProviderCredentials)
	if err != nil {
		return nil, fmt.Errorf("auth provider credentials encryption: %w", err)
	}
	return &AuthProviderEncryptionService{enc: enc}, nil
}

// Encrypt marshals v as JSON and encrypts the result.
// v must be one of the typed provider config structs (OIDCConfig, FusionAuthConfig, LDAPConfig).
func (s *AuthProviderEncryptionService) Encrypt(v any) ([]byte, error) {
	b, err := marshalRaw(v)
	if err != nil {
		return nil, fmt.Errorf("encrypt auth provider config: marshal: %w", err)
	}
	ct, err := s.enc.Encrypt(b)
	if err != nil {
		return nil, fmt.Errorf("encrypt auth provider config: %w", err)
	}
	return ct, nil
}

// Decrypt decrypts ciphertext and unmarshals the result into target.
// target must be a pointer to one of the typed provider config structs.
func (s *AuthProviderEncryptionService) Decrypt(ciphertext []byte, target any) error {
	b, err := s.enc.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt auth provider config: %w", err)
	}
	if err := unmarshalRaw(b, target); err != nil {
		return fmt.Errorf("decrypt auth provider config: unmarshal: %w", err)
	}
	return nil
}
