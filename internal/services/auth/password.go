// Package auth provides authentication services for the Tackle platform.
package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword hashes the given plaintext password using bcrypt at cost 12.
// The plaintext password is never stored or logged.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// ComparePassword reports whether plaintext password matches the bcrypt hash.
// Uses constant-time comparison to prevent timing attacks.
// Returns nil on match, non-nil error on mismatch or invalid hash.
func ComparePassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("compare password: %w", err)
	}
	return nil
}
