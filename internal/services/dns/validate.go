// Package dns implements DNS record management, validation, propagation checking,
// and email authentication record building.
package dns

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	dnsiface "tackle/internal/providers/dns"
)

// ValidationError is returned when one or more DNS record fields fail validation.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("dns validation: %s: %s", e.Field, e.Message)
}

// ValidationWarning is a non-fatal advisory message (e.g. TTL out of recommended range).
type ValidationWarning struct {
	Field   string
	Message string
}

// ValidationResult holds both errors and warnings from record validation.
type ValidationResult struct {
	Errors   []*ValidationError
	Warnings []ValidationWarning
}

// OK returns true when there are no validation errors (warnings are allowed).
func (v *ValidationResult) OK() bool { return len(v.Errors) == 0 }

// hostnameRE matches RFC 1123 hostnames: labels of 1–63 alphanumeric/hyphen chars,
// separated by dots, with no leading or trailing hyphens per label.
var hostnameRE = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// recordNameRE allows underscores and wildcards commonly used in DNS record names (e.g., _dmarc, *).
var recordNameRE = regexp.MustCompile(`^([a-zA-Z0-9\*\_\-]{1,63}\.)*[a-zA-Z0-9\*\_\-]{1,63}$`)

const (
	minTTL     = 60
	maxTTL     = 86400
	maxMXPrio  = 65535
	maxLabelLen = 63
	maxTXTChunkLen = 255
)

// ValidateRecord validates a DNS record and returns a ValidationResult containing
// any field-level errors and advisory warnings.
func ValidateRecord(r dnsiface.Record) ValidationResult {
	var result ValidationResult

	validateName(r.Name, &result)
	validateTTL(r.TTL, &result)

	switch r.Type {
	case dnsiface.RecordTypeA:
		validateA(r.Value, &result)
	case dnsiface.RecordTypeAAAA:
		validateAAAA(r.Value, &result)
	case dnsiface.RecordTypeCNAME:
		validateCNAME(r.Name, r.Value, &result)
	case dnsiface.RecordTypeMX:
		validateMX(r.Value, r.Priority, &result)
	case dnsiface.RecordTypeTXT:
		validateTXT(r.Value, &result)
	case dnsiface.RecordTypeNS:
		validateNS(r.Value, &result)
	case dnsiface.RecordTypeSOA:
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "type",
			Message: "SOA records are read-only and cannot be created or modified",
		})
	default:
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("unsupported record type %q", r.Type),
		})
	}

	return result
}

func validateName(name string, result *ValidationResult) {
	if name == "" {
		result.Errors = append(result.Errors, &ValidationError{Field: "name", Message: "name is required"})
		return
	}
	if name == "@" {
		return // zone apex — always valid
	}

	labels := strings.Split(name, ".")
	for _, label := range labels {
		if len(label) > maxLabelLen {
			result.Errors = append(result.Errors, &ValidationError{
				Field:   "name",
				Message: fmt.Sprintf("label %q exceeds 63-character limit", label),
			})
			return
		}
	}

	if !recordNameRE.MatchString(name) {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "name",
			Message: fmt.Sprintf("name %q contains invalid characters (allowed: alphanumeric, -, _, *)", name),
		})
	}
}

func validateTTL(ttl int, result *ValidationResult) {
	if ttl <= 0 {
		result.Errors = append(result.Errors, &ValidationError{Field: "ttl", Message: "TTL must be a positive integer"})
		return
	}
	if ttl < minTTL || ttl > maxTTL {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "ttl",
			Message: fmt.Sprintf("TTL %d is outside recommended range %d–%d", ttl, minTTL, maxTTL),
		})
	}
}

func validateA(value string, result *ValidationResult) {
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "value",
			Message: fmt.Sprintf("value %q is not a valid IPv4 address", value),
		})
	}
}

func validateAAAA(value string, result *ValidationResult) {
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() != nil {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "value",
			Message: fmt.Sprintf("value %q is not a valid IPv6 address", value),
		})
	}
}

func validateCNAME(name, value string, result *ValidationResult) {
	// CNAME cannot be at zone apex.
	if name == "@" {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "name",
			Message: "CNAME records cannot be placed at the zone apex (@)",
		})
	}
	if !isValidHostname(value) {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "value",
			Message: fmt.Sprintf("value %q is not a valid hostname", value),
		})
	}
}

func validateMX(value string, priority int, result *ValidationResult) {
	if !isValidHostname(value) {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "value",
			Message: fmt.Sprintf("value %q is not a valid hostname", value),
		})
	}
	if priority < 0 || priority > maxMXPrio {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "priority",
			Message: fmt.Sprintf("MX priority must be 0–%d, got %d", maxMXPrio, priority),
		})
	}
}

func validateTXT(value string, result *ValidationResult) {
	if len(value) == 0 {
		result.Errors = append(result.Errors, &ValidationError{Field: "value", Message: "TXT value cannot be empty"})
		return
	}
	// Single TXT string segments must be ≤ 255 bytes. Long values must be split.
	// We warn rather than error because the service layer handles splitting.
	if len(value) > maxTXTChunkLen {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "value",
			Message: fmt.Sprintf("TXT value exceeds 255 characters (%d); it will be split into multiple strings automatically", len(value)),
		})
	}
}

func validateNS(value string, result *ValidationResult) {
	if !isValidHostname(value) {
		result.Errors = append(result.Errors, &ValidationError{
			Field:   "value",
			Message: fmt.Sprintf("value %q is not a valid hostname", value),
		})
	}
}

// isValidHostname returns true if s is a valid RFC 1123 hostname (bare or FQDN).
func isValidHostname(s string) bool {
	if s == "" {
		return false
	}
	// Strip optional trailing dot (FQDN).
	s = strings.TrimSuffix(s, ".")
	return hostnameRE.MatchString(s)
}

// SplitTXTValue splits a TXT record value into RFC 4408-compliant 255-byte chunks.
func SplitTXTValue(value string) []string {
	if len(value) <= maxTXTChunkLen {
		return []string{value}
	}

	var chunks []string
	for len(value) > 0 {
		end := maxTXTChunkLen
		if end > len(value) {
			end = len(value)
		}
		chunks = append(chunks, value[:end])
		value = value[end:]
	}
	return chunks
}
