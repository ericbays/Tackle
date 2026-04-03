package credentials

import (
	"fmt"

	"tackle/internal/crypto"
)

// PurposeSMTPCredentials is the HKDF purpose string for SMTP credential encryption.
const PurposeSMTPCredentials = "tackle/smtp-credentials"

// SMTPEncryptionService encrypts and decrypts individual SMTP credential strings
// (username and password are encrypted separately).
type SMTPEncryptionService struct {
	enc *crypto.EncryptionService
}

// NewSMTPEncryptionService creates an SMTPEncryptionService from the platform master key.
func NewSMTPEncryptionService(masterKey []byte) (*SMTPEncryptionService, error) {
	enc, err := crypto.NewEncryptionServiceForPurpose(masterKey, PurposeSMTPCredentials)
	if err != nil {
		return nil, fmt.Errorf("smtp credentials encryption: %w", err)
	}
	return &SMTPEncryptionService{enc: enc}, nil
}

// Encrypt encrypts a plaintext credential string using AES-256-GCM.
func (s *SMTPEncryptionService) Encrypt(value string) ([]byte, error) {
	ct, err := s.enc.Encrypt([]byte(value))
	if err != nil {
		return nil, fmt.Errorf("smtp credentials: encrypt: %w", err)
	}
	return ct, nil
}

// Decrypt decrypts a ciphertext produced by Encrypt and returns the plaintext string.
func (s *SMTPEncryptionService) Decrypt(ciphertext []byte) (string, error) {
	pt, err := s.enc.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("smtp credentials: decrypt: %w", err)
	}
	return string(pt), nil
}

// SMTPCredential is a single encrypted credential value (username or password).
// MarshalJSON always returns the masked representation so it is safe to include
// in API responses, log entries, and error messages.
type SMTPCredential struct {
	value string
}

// NewSMTPCredential wraps a plaintext value in an SMTPCredential.
func NewSMTPCredential(v string) SMTPCredential { return SMTPCredential{value: v} }

// Value returns the plaintext credential value.
// This must only be called when the value is needed for an actual SMTP connection.
func (c SMTPCredential) Value() string { return c.value }

// MarshalJSON returns the masked representation.
func (c SMTPCredential) MarshalJSON() ([]byte, error) { return []byte(`"` + masked + `"`), nil }

// String returns the masked representation.
func (c SMTPCredential) String() string { return "<SMTPCredential masked>" }
