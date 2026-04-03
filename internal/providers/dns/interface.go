// Package dns defines the common interface and types for DNS record management
// across all supported DNS providers (Namecheap, GoDaddy, Route 53, Azure DNS).
package dns

import (
	"context"
	"errors"
	"fmt"
)

// RecordType enumerates the supported DNS record types.
type RecordType string

const (
	// RecordTypeA is an IPv4 address record.
	RecordTypeA RecordType = "A"
	// RecordTypeAAAA is an IPv6 address record.
	RecordTypeAAAA RecordType = "AAAA"
	// RecordTypeCNAME is a canonical name (alias) record.
	RecordTypeCNAME RecordType = "CNAME"
	// RecordTypeMX is a mail exchange record.
	RecordTypeMX RecordType = "MX"
	// RecordTypeTXT is a text record.
	RecordTypeTXT RecordType = "TXT"
	// RecordTypeNS is a name server record.
	RecordTypeNS RecordType = "NS"
	// RecordTypeSOA is a start-of-authority record (read-only).
	RecordTypeSOA RecordType = "SOA"
)

// Record is the provider-agnostic representation of a DNS record.
type Record struct {
	// ID is a provider-specific identifier for the record. May be empty for
	// providers that address records by type+name rather than by ID.
	ID string

	// Type is the DNS record type (A, AAAA, CNAME, MX, TXT, NS, SOA).
	Type RecordType

	// Name is the record name / subdomain (e.g. "@", "www", "mail").
	// Use "@" for the zone apex.
	Name string

	// Value is the record data (e.g. IP address, hostname, TXT string).
	Value string

	// TTL is the time-to-live in seconds.
	TTL int

	// Priority is used for MX records. Zero for all other record types.
	Priority int
}

// Provider is the interface that all DNS provider clients must implement.
// All methods accept a zone argument which is the bare domain name (e.g. "example.com").
type Provider interface {
	// ListRecords returns all DNS records in the zone.
	ListRecords(ctx context.Context, zone string) ([]Record, error)

	// CreateRecord creates a new DNS record in the zone and returns the created
	// record, including any provider-assigned ID.
	CreateRecord(ctx context.Context, zone string, record Record) (Record, error)

	// UpdateRecord replaces the DNS record identified by recordID with the new
	// record data. For providers that address records by type+name, recordID
	// encodes both (e.g. "A/www").
	UpdateRecord(ctx context.Context, zone string, recordID string, record Record) (Record, error)

	// DeleteRecord removes the DNS record identified by recordID from the zone.
	DeleteRecord(ctx context.Context, zone string, recordID string) error

	// GetSOA returns the SOA record for the zone. This is read-only — SOA
	// records cannot be created or deleted through this interface.
	GetSOA(ctx context.Context, zone string) (Record, error)
}

// Sentinel errors returned by Provider implementations.
var (
	// ErrRecordNotFound is returned when the requested DNS record does not exist.
	ErrRecordNotFound = errors.New("dns: record not found")

	// ErrRecordConflict is returned when a record already exists and the
	// provider does not allow duplicates of that type/name combination.
	ErrRecordConflict = errors.New("dns: record conflict")

	// ErrZoneNotFound is returned when the specified zone does not exist in
	// the provider account.
	ErrZoneNotFound = errors.New("dns: zone not found")

	// ErrAPIRateLimit is returned when the provider API rate limit is exceeded.
	ErrAPIRateLimit = errors.New("dns: API rate limit exceeded")
)

// RecordTypeFromString parses a string into a RecordType, returning an error
// if the type is not recognised.
func RecordTypeFromString(s string) (RecordType, error) {
	switch RecordType(s) {
	case RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeMX,
		RecordTypeTXT, RecordTypeNS, RecordTypeSOA:
		return RecordType(s), nil
	default:
		return "", fmt.Errorf("dns: unknown record type %q", s)
	}
}
