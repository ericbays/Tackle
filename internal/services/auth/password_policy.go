package auth

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

//go:embed breached_passwords.txt
var breachedPasswordsRaw string

// breachedSet is built once at init from the embedded list.
var breachedSet map[string]struct{}

func init() {
	lines := strings.Split(breachedPasswordsRaw, "\n")
	breachedSet = make(map[string]struct{}, len(lines))
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			breachedSet[strings.ToLower(trimmed)] = struct{}{}
		}
	}
}

// PasswordPolicy defines complexity requirements for local account passwords.
type PasswordPolicy struct {
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireDigit     bool
	RequireSpecial   bool
}

// DefaultPolicy returns the default password policy per REQ-AUTH-020.
func DefaultPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:        12,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
	}
}

// Validate checks whether password satisfies the policy.
// Returns an error describing the first violation found, or nil if valid.
func (p PasswordPolicy) Validate(password string) error {
	if len(password) < p.MinLength {
		return fmt.Errorf("password must be at least %d characters long", p.MinLength)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	var errs []string
	if p.RequireUppercase && !hasUpper {
		errs = append(errs, "at least one uppercase letter")
	}
	if p.RequireLowercase && !hasLower {
		errs = append(errs, "at least one lowercase letter")
	}
	if p.RequireDigit && !hasDigit {
		errs = append(errs, "at least one digit")
	}
	if p.RequireSpecial && !hasSpecial {
		errs = append(errs, "at least one special character")
	}
	if len(errs) > 0 {
		return errors.New("password must contain " + strings.Join(errs, ", "))
	}
	return nil
}

// IsBreached reports whether the password appears in the embedded breached-password list.
// Comparison is case-insensitive.
func IsBreached(password string) bool {
	_, found := breachedSet[strings.ToLower(password)]
	return found
}
