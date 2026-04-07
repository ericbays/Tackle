// Package providers defines the ProviderClient interface and shared utilities
// used by domain provider implementations.
package providers

// ProviderClient is the common interface that all domain provider clients must implement.
type ProviderClient interface {
	// TestConnection validates the stored credentials against the provider's API.
	// Returns nil on success or a descriptive, actionable error on failure.
	TestConnection() error

	// ListDomains retrieves a list of all domains associated with the provider.
	// Returns a slice of fully qualified domain names or an error.
	ListDomains() ([]string, error)
}
