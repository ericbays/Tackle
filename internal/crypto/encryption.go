package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
)

const (
	keySize   = 32 // AES-256 requires a 32-byte key.
	nonceSize = 12 // GCM standard nonce length.
)

// EncryptionService provides AES-256-GCM authenticated encryption and decryption.
// Each instance is bound to a single key. Use NewEncryptionService or
// NewEncryptionServiceForPurpose to construct one.
type EncryptionService struct {
	key []byte
}

// NewEncryptionService creates an EncryptionService using key directly.
// key must be exactly 32 bytes; any other length returns an error.
func NewEncryptionService(key []byte) (*EncryptionService, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("encryption service: key must be exactly %d bytes, got %d", keySize, len(key))
	}
	k := make([]byte, keySize)
	copy(k, key)
	return &EncryptionService{key: k}, nil
}

// NewEncryptionServiceForPurpose creates an EncryptionService using a subkey
// derived from masterKey via HKDF. purpose must be one of the Purpose* constants.
func NewEncryptionServiceForPurpose(masterKey []byte, purpose string) (*EncryptionService, error) {
	subkey, err := DeriveSubkey(masterKey, purpose)
	if err != nil {
		return nil, fmt.Errorf("encryption service: %w", err)
	}
	return NewEncryptionService(subkey)
}

// Encrypt encrypts plaintext using AES-256-GCM with a randomly generated nonce.
// The wire format is: nonce (12 bytes) || GCM ciphertext+tag.
func (s *EncryptionService) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt: create gcm: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encrypt: generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag after nonce.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext produced by Encrypt. Returns an error if the GCM
// authentication tag fails — indicating tampering or use of a wrong key.
func (s *EncryptionService) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("decrypt: ciphertext too short")
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decrypt: create gcm: %w", err)
	}

	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: authentication failed: %w", err)
	}
	return plaintext, nil
}

// EncryptString encrypts a UTF-8 string and returns the ciphertext bytes.
func (s *EncryptionService) EncryptString(plaintext string) ([]byte, error) {
	return s.Encrypt([]byte(plaintext))
}

// DecryptString decrypts ciphertext and returns the plaintext as a string.
func (s *EncryptionService) DecryptString(ciphertext []byte) (string, error) {
	b, err := s.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// EncryptJSON marshals value to JSON and then encrypts the result.
// Useful for encrypting structured JSONB columns.
func (s *EncryptionService) EncryptJSON(value any) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encrypt json: marshal: %w", err)
	}
	ciphertext, err := s.Encrypt(b)
	if err != nil {
		return nil, fmt.Errorf("encrypt json: %w", err)
	}
	return ciphertext, nil
}

// DecryptJSON decrypts ciphertext and unmarshals the result into target.
func (s *EncryptionService) DecryptJSON(ciphertext []byte, target any) error {
	b, err := s.Decrypt(ciphertext)
	if err != nil {
		return fmt.Errorf("decrypt json: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("decrypt json: unmarshal: %w", err)
	}
	return nil
}
